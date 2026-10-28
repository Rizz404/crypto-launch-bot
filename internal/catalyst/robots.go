package catalyst

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const collectorAgent = "trading-bot-catalyst-research"

func (c *Collector) checkRobots(ctx context.Context, source string) error {
	u, err := url.Parse(source)
	if err != nil {
		return errors.New("invalid robots origin")
	}
	path := u.EscapedPath()
	u.Path, u.RawPath = "/robots.txt", ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return errors.New("invalid robots request")
	}
	req.Header.Set("User-Agent", collectorAgent+"/1.0")
	resp, err := c.client.Do(req)
	if err != nil {
		return errors.New("robots unavailable")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("robots check failed")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10+1))
	if err != nil || len(body) > 64<<10 {
		return errors.New("robots unreadable or too large")
	}
	if !robotsAllow(string(body), path) {
		return errors.New("robots disallows source")
	}
	return nil
}

func robotsAllow(body, path string) bool {
	type rule struct {
		path  string
		allow bool
	}
	type group struct {
		agents []string
		rules  []rule
	}
	var groups []group
	current := group{}
	flush := func() {
		if len(current.agents) > 0 {
			groups = append(groups, current)
		}
		current = group{}
	}
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line, _, _ := strings.Cut(scanner.Text(), "#")
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
		if key == "user-agent" {
			if len(current.rules) > 0 {
				flush()
			}
			current.agents = append(current.agents, strings.ToLower(value))
		} else if (key == "allow" || key == "disallow") && len(current.agents) > 0 && value != "" {
			current.rules = append(current.rules, rule{value, key == "allow"})
		}
	}
	if scanner.Err() != nil {
		return false
	}
	flush()
	bestAgent := -1
	var rules []rule
	for _, g := range groups {
		score := -1
		for _, agent := range g.agents {
			if agent == "*" {
				score = max(score, 0)
			} else if agent != "" && strings.Contains(collectorAgent, agent) {
				score = max(score, len(agent))
			}
		}
		if score < 0 || score < bestAgent {
			continue
		}
		if score > bestAgent {
			rules = nil
			bestAgent = score
		}
		rules = append(rules, g.rules...)
	}
	allowed, best := true, -1
	for _, r := range rules {
		pattern := regexp.QuoteMeta(r.path)
		pattern = strings.ReplaceAll(pattern, `\*`, ".*")
		if strings.HasSuffix(pattern, `\$`) {
			pattern = strings.TrimSuffix(pattern, `\$`) + "$"
		}
		re, err := regexp.Compile("^" + pattern)
		if err != nil {
			return false
		}
		length := len(strings.ReplaceAll(r.path, "*", ""))
		if re.MatchString(path) && (length > best || (length == best && r.allow)) {
			best, allowed = length, r.allow
		}
	}
	return allowed
}
