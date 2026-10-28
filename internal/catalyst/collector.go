package catalyst

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"
)

type Collector struct {
	Policy       Policy
	Archive      Archive
	client       *http.Client
	minInterval  time.Duration
	spiderBinary string
}

func NewCollector(p Policy, archive Archive) (*Collector, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            publicDial,
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  15 * time.Second,
		MaxResponseHeaderBytes: 32 << 10,
		MaxIdleConns:           2,
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("redirect denied; configure the final source URL explicitly")
		},
	}
	return &Collector{Policy: p, Archive: archive, client: client, minInterval: time.Duration(p.PollSeconds) * time.Second}, nil
}

func publicIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "2002::/16"} {
		if netip.MustParsePrefix(prefix).Contains(addr) {
			return false
		}
	}
	return true
}

// Resolve once and dial the validated IP, closing the DNS-rebinding window.
func publicDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errors.New("invalid source address")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("source DNS lookup failed")
	}
	for _, ip := range ips {
		if !publicIP(ip.IP) {
			return nil, errors.New("source resolves to a non-public address")
		}
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

type Observation struct {
	SourceID string    `json:"source_id"`
	At       time.Time `json:"at"`
	Evidence string    `json:"evidence_id,omitempty"`
	Status   string    `json:"status"`
	HTTP     int       `json:"http_status,omitempty"`
}

func (c *Collector) Collect(ctx context.Context, source Source) (Evidence, error) {
	if !slices.ContainsFunc(c.Policy.Sources, func(s Source) bool { return Hash(s) == Hash(source) }) {
		return Evidence{}, errors.New("source outside configured policy")
	}
	obs := Observation{SourceID: source.ID, At: time.Now().UTC(), Status: "failed"}
	ids, err := c.Archive.IDs("observations")
	if err != nil {
		return Evidence{}, err
	}
	var previous Observation
	for _, id := range ids {
		var old Observation
		if err = c.Archive.Read("observations", id, &old); err != nil {
			return Evidence{}, err
		}
		if old.SourceID == source.ID && old.At.After(previous.At) {
			previous = old
		}
	}
	if !previous.At.IsZero() && obs.At.Sub(previous.At) < c.minInterval {
		if previous.Status != "ok" {
			return Evidence{}, errors.New("source backoff active")
		}
		ids, err = c.Archive.IDs("evidence")
		if err != nil {
			return Evidence{}, err
		}
		version := ExtractionVersion
		if c.spiderBinary != "" {
			version = spiderExtractionVersion
		}
		for _, id := range ids {
			var cached Evidence
			if err = c.Archive.Read("evidence", id, &cached); err != nil {
				return Evidence{}, err
			}
			if cached.ID == previous.Evidence && cached.SourceID == source.ID && cached.URL == source.URL && cached.ExtractionVersion == version {
				return cached, nil
			}
		}
		return Evidence{}, errors.New("cached source evidence missing")
	}
	e, status, fetchErr := c.fetch(ctx, source)
	obs.HTTP = status
	if fetchErr == nil {
		// Content-addressed bodies retain the earliest local receipt, including
		// when a later source revision returns to an earlier body.
		ids, err := c.Archive.IDs("evidence")
		if err != nil {
			return Evidence{}, err
		}
		for _, id := range ids {
			var old Evidence
			if err = c.Archive.Read("evidence", id, &old); err != nil {
				return Evidence{}, err
			}
			if old.ID == e.ID && old.FirstSeenAt.Before(e.FirstSeenAt) {
				e.FirstSeenAt = old.FirstSeenAt
				e.Raw = old.Raw // Volatile HTTP headers do not create another content revision.
			}
			for n := range e.Documents {
				for _, doc := range old.Documents {
					if doc.ID == e.Documents[n].ID && doc.Revision == e.Documents[n].Revision && doc.FirstSeenAt.Before(e.Documents[n].FirstSeenAt) {
						e.Documents[n].FirstSeenAt = doc.FirstSeenAt
					}
				}
			}
		}
		if _, err = c.Archive.Put("evidence", e); err != nil {
			fetchErr = err
		} else {
			obs.Status, obs.Evidence = "ok", e.ID
		}
	}
	if _, err := c.Archive.Put("observations", obs); err != nil {
		return Evidence{}, err
	}
	return e, fetchErr
}

func (c *Collector) fetch(ctx context.Context, source Source) (Evidence, int, error) {
	if _, err := publicURL(source.URL); err != nil {
		return Evidence{}, 0, err
	}
	if err := c.checkRobots(ctx, source.URL); err != nil {
		return Evidence{}, 0, err
	}
	if c.spiderBinary != "" {
		return c.fetchSpider(ctx, source)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return Evidence{}, 0, errors.New("invalid source request")
	}
	req.Header.Set("User-Agent", "trading-bot-catalyst-research/1.0")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/json, text/html, text/plain")
	resp, err := c.client.Do(req)
	if err != nil {
		return Evidence{}, 0, errors.New("source fetch failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return Evidence{}, resp.StatusCode, fmt.Errorf("source HTTP %d", resp.StatusCode)
	}
	media, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if !slices.Contains([]string{"text/html", "text/plain", "application/json", "application/rss+xml", "application/atom+xml", "application/xml", "text/xml"}, media) {
		return Evidence{}, resp.StatusCode, errors.New("unsupported source content type")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, MaxDocumentBytes+1))
	if err != nil || len(body) == 0 || len(body) > MaxDocumentBytes {
		return Evidence{}, resp.StatusCode, errors.New("source empty, unreadable or too large")
	}
	now := time.Now().UTC()
	docs, text, err := extractDocuments(source, media, string(body), now)
	if err != nil {
		return Evidence{}, resp.StatusCode, err
	}
	e := Evidence{SourceID: source.ID, URL: source.URL, BodyHash: fmt.Sprintf("%x", sha256.Sum256(body)), ContentType: media, FirstSeenAt: now, Body: text, Raw: string(body), ExtractionVersion: ExtractionVersion, Documents: docs}
	e.ID = Hash([]string{source.ID, source.URL, e.BodyHash, ExtractionVersion})
	return e, resp.StatusCode, nil
}

// LatestEvidence uses observation order rather than first_seen: a page may
// revert to an old body, or its latest fetch may have failed.
func (a Archive) LatestEvidence(p Policy) ([]Evidence, error) {
	out, _, err := a.latestEvidence(p, false)
	return out, err
}

// AvailableEvidence records source gaps without discarding healthy assets.
// Corrupt/unreadable archives remain fatal; missing sources cannot attest entry.
func (a Archive) AvailableEvidence(p Policy) ([]Evidence, []string, error) {
	return a.latestEvidence(p, true)
}

func (a Archive) latestEvidence(p Policy, partial bool) ([]Evidence, []string, error) {
	ids, err := a.IDs("observations")
	if err != nil {
		return nil, nil, err
	}
	latest := map[string]Observation{}
	for _, id := range ids {
		var obs Observation
		if err = a.Read("observations", id, &obs); err != nil {
			return nil, nil, err
		}
		if previous, ok := latest[obs.SourceID]; !ok || obs.At.After(previous.At) {
			latest[obs.SourceID] = obs
		}
	}
	ids, err = a.IDs("evidence")
	if err != nil {
		return nil, nil, err
	}
	var out []Evidence
	var gaps []string
	for _, source := range p.Sources {
		obs := latest[source.ID]
		if obs.Status != "ok" || time.Since(obs.At) > time.Duration(p.PollSeconds*2)*time.Second || obs.At.After(time.Now()) {
			if partial {
				gaps = append(gaps, source.ID)
				continue
			}
			return nil, nil, errors.New("source missing, stale or latest fetch failed: " + source.ID)
		}
		found := false
		var selected Evidence
		for _, id := range ids {
			var e Evidence
			if err = a.Read("evidence", id, &e); err != nil {
				return nil, nil, err
			}
			if e.SourceID == source.ID && e.URL == source.URL && e.ID == obs.Evidence {
				if !found || e.FirstSeenAt.Before(selected.FirstSeenAt) {
					selected = e
				}
				found = true
			}
		}
		if !found {
			if partial {
				gaps = append(gaps, source.ID)
				continue
			}
			return nil, nil, errors.New("source evidence missing")
		}
		out = append(out, selected)
	}
	slices.SortFunc(out, func(a, b Evidence) int { return strings.Compare(a.ID, b.ID) })
	return out, gaps, nil
}
