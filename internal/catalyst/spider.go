package catalyst

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const SpiderVersion = "2.53.9-p3.1"
const spiderExtractionVersion = ExtractionVersion + "/content-v2/spider-" + SpiderVersion
const spiderPages = 5
const spiderOutputBytes = 8 << 20

// NewSpiderCollector makes Spider the collection engine; it never silently
// falls back to the development HTTP collector when Spider is unavailable.
func NewSpiderCollector(p Policy, archive Archive, binary string) (*Collector, error) {
	c, err := NewCollector(p, archive)
	if err != nil {
		return nil, err
	}
	if binary == "" {
		return nil, errors.New("spider binary required")
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return nil, errors.New("spider is not installed; use the dedicated catalyst image")
	}
	c.spiderBinary = path
	return c, nil
}

type spiderPage struct {
	URL        string            `json:"url"`
	Content    string            `json:"content"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers"`
	Links      json.RawMessage   `json:"links"`
}

type boundedOutput struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (b *boundedOutput) Write(data []byte) (int, error) {
	if len(data) > spiderOutputBytes-b.Len() {
		b.cancel()
		return 0, errors.New("spider output exceeds limit")
	}
	return b.Buffer.Write(data)
}

func (c *Collector) fetchSpider(ctx context.Context, source Source) (Evidence, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	proxy, closeProxy, err := spiderProxy(ctx, source.URL)
	if err != nil {
		return Evidence{}, 0, err
	}
	defer closeProxy()
	version := &boundedOutput{cancel: cancel}
	check := exec.CommandContext(ctx, c.spiderBinary, "--version")
	check.Env = []string{"RUST_LOG=error"}
	check.Stdout = version
	check.WaitDelay = time.Second
	if err = check.Run(); err != nil || strings.TrimSpace(version.String()) != "spider_cli "+SpiderVersion {
		return Evidence{}, 0, errors.New("spider version must be " + SpiderVersion)
	}
	output := &boundedOutput{cancel: cancel}
	if source.ArticlePattern == "" {
		if err = c.runSpider(ctx, source.URL, spiderPages, proxy, output); err != nil {
			return Evidence{}, 0, err
		}
		return spiderEvidence(source, output.Bytes(), time.Now().UTC())
	}
	// Spider's depth counts path segments and its crawl order is concurrent, so
	// dated article paths are selected from the seed and scraped one by one.
	if err = c.runSpider(ctx, source.URL, 1, proxy, output); err != nil {
		return Evidence{}, 0, err
	}
	links, err := articleLinks(source, output.Bytes())
	if err != nil {
		return Evidence{}, 0, err
	}
	for _, link := range links {
		select {
		case <-ctx.Done():
			return Evidence{}, 0, errors.New("spider timed out between article pages")
		case <-time.After(time.Second):
		}
		if err = c.runSpider(ctx, link, 1, proxy, output); err != nil {
			return Evidence{}, 0, err
		}
	}
	return spiderEvidence(source, output.Bytes(), time.Now().UTC())
}

// runSpider appends one scrape to output, whose limit spans every run of a fetch.
func (c *Collector) runSpider(ctx context.Context, target string, limit int, proxy string, output *boundedOutput) error {
	cmd := exec.CommandContext(ctx, c.spiderBinary,
		"--url", target, "--respect-robots-txt", "--http",
		"--return-headers", "--limit", fmt.Sprint(limit), "--depth", "2",
		"--delay", "1000",
		"--agent", collectorAgent+"/1.0", "--proxy-url", proxy,
		"scrape", "--output-html")
	// Scraping receives no parent credentials, proxy settings or cloud keys.
	cmd.Env = []string{"RUST_LOG=error"}
	cmd.WaitDelay = time.Second
	cmd.Stdout = output
	cmd.Stderr = io.Discard // Upstream diagnostics may contain untrusted URLs.
	if err := cmd.Run(); err != nil {
		return errors.New("spider failed, timed out or exceeded output limit")
	}
	return nil
}

// articleLinks returns up to spiderPages-1 same-host links from the seed page
// whose path matches the source pattern, in seed-page order.
func articleLinks(source Source, output []byte) ([]string, error) {
	pattern, err := regexp.Compile(source.ArticlePattern)
	if err != nil {
		return nil, errors.New("invalid source article pattern")
	}
	var seed spiderPage
	if err = json.NewDecoder(bytes.NewReader(output)).Decode(&seed); err != nil {
		return nil, errors.New("invalid Spider JSON stream")
	}
	base, err := publicURL(seed.URL)
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(strings.NewReader(seed.Content))
	if err != nil {
		return nil, errors.New("seed page is not parseable HTML")
	}
	seen := map[string]bool{}
	var links []string
	for n := range doc.Descendants() {
		if n.Type != html.ElementNode || n.Data != "a" {
			continue
		}
		for _, a := range n.Attr {
			if a.Key != "href" {
				continue
			}
			u, err := base.Parse(strings.TrimSpace(a.Val))
			if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), base.Hostname()) || !pattern.MatchString(u.Path) {
				continue
			}
			u.RawQuery, u.Fragment = "", ""
			if link := u.String(); !seen[link] {
				seen[link] = true
				links = append(links, link)
			}
		}
	}
	if len(links) == 0 {
		return nil, errors.New("seed page has no article links; coverage incomplete")
	}
	// shortcut: seed order is trusted as newest first, as the configured index pages list posts; use published dates if a source does not.
	return links[:min(len(links), spiderPages-1)], nil
}

func spiderEvidence(source Source, output []byte, now time.Time) (Evidence, int, error) {
	if len(output) > spiderOutputBytes {
		return Evidence{}, 0, errors.New("spider output exceeds limit")
	}
	base, err := publicURL(source.URL)
	if err != nil {
		return Evidence{}, 0, err
	}
	dec := json.NewDecoder(bytes.NewReader(output))
	var pages []spiderPage
	seen, foundSeed := map[string]bool{}, false
	for {
		var raw json.RawMessage
		if err = dec.Decode(&raw); err == io.EOF {
			break
		} else if err != nil {
			return Evidence{}, 0, errors.New("invalid Spider JSON stream")
		}
		if len(pages) >= spiderPages {
			return Evidence{}, 0, errors.New("spider page budget exceeded")
		}
		var page spiderPage
		if err = Decode(bytes.NewReader(raw), &page, spiderOutputBytes); err != nil {
			return Evidence{}, 0, errors.New("spider output does not match the pinned protocol")
		}
		u, err := publicURL(page.URL)
		if err != nil || !strings.EqualFold(u.Hostname(), base.Hostname()) || seen[page.URL] {
			return Evidence{}, 0, errors.New("spider returned a duplicate or out-of-origin URL")
		}
		seen[page.URL] = true
		foundSeed = foundSeed || strings.TrimRight(page.URL, "/") == strings.TrimRight(source.URL, "/")
		if page.StatusCode != 200 {
			return Evidence{}, page.StatusCode, fmt.Errorf("spider page HTTP %d; coverage incomplete", page.StatusCode)
		}
		pages = append(pages, page)
	}
	if !foundSeed {
		return Evidence{}, 0, errors.New("spider did not return the configured seed page")
	}
	// Stable ordering avoids a new research revision due to crawler arrival order.
	slices.SortFunc(pages, func(a, b spiderPage) int { return strings.Compare(a.URL, b.URL) })
	e := Evidence{SourceID: source.ID, URL: source.URL, ContentType: "application/vnd.spider+json", FirstSeenAt: now, ExtractionVersion: spiderExtractionVersion}
	var text strings.Builder
	var content []string
	documents := map[string]bool{}
	for _, page := range pages {
		var contentType string
		for k, v := range page.Headers {
			if strings.EqualFold(k, "Content-Type") {
				contentType = v
			}
		}
		media, _, _ := mime.ParseMediaType(contentType)
		if !slices.Contains([]string{"text/html", "text/plain", "application/json", "application/rss+xml", "application/atom+xml", "application/xml", "text/xml"}, media) {
			return Evidence{}, 200, errors.New("spider page has no supported content type")
		}
		pageSource := source
		pageSource.URL = page.URL
		docs, body, err := extractDocuments(pageSource, media, page.Content, now)
		if err != nil {
			return Evidence{}, 200, err
		}
		content = append(content, page.URL, media, body)
		for _, d := range docs {
			content = append(content, d.ID, d.Revision)
			if documents[d.ID] {
				return Evidence{}, 200, errors.New("overlapping Spider documents; narrow the source")
			}
			documents[d.ID] = true
			e.Documents = append(e.Documents, d)
		}
		text.WriteString("Fetched page: " + page.URL + "\n" + body + "\n\n")
	}
	if text.Len() > MaxDocumentBytes {
		return Evidence{}, 200, errors.New("spider extracted text exceeds source budget")
	}
	raw, err := json.Marshal(pages)
	if err != nil {
		return Evidence{}, 200, err
	}
	e.Raw, e.Body = string(raw), strings.TrimSpace(text.String())
	e.BodyHash = Hash(content)
	e.ID = Hash([]string{source.ID, source.URL, e.BodyHash, e.ExtractionVersion})
	record, err := json.Marshal(e)
	if err != nil || len(record) > maxArchiveBytes {
		return Evidence{}, 200, errors.New("spider evidence exceeds archive limit")
	}
	return e, 200, nil
}

// sourceHost extracts only a configured host; it never accepts discovered URLs.
func sourceHost(source string) (string, error) {
	u, err := publicURL(source)
	if err != nil {
		return "", errors.New("invalid Spider source host")
	}
	return u.Hostname(), nil
}
