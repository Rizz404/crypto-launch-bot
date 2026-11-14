package catalyst

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func spiderJSON(t *testing.T, pages ...spiderPage) []byte {
	t.Helper()
	var out bytes.Buffer
	for _, page := range pages {
		require.NoError(t, json.NewEncoder(&out).Encode(page))
	}
	return out.Bytes()
}

// fakeSpiderFixture drives a copy of this test binary acting as the Spider CLI,
// so subprocess tests run on every OS without a shell.
type fakeSpiderFixture struct {
	Version string            `json:"version"`
	Limit   string            `json:"limit"`
	Pages   map[string]string `json:"pages"`
	Calls   string            `json:"calls,omitempty"`
}

const fakeSpiderName = "fake-spider"

func TestMain(m *testing.M) {
	if strings.HasPrefix(filepath.Base(os.Args[0]), fakeSpiderName) {
		os.Exit(runFakeSpider(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func runFakeSpider(args []string) int {
	self, err := os.Executable()
	if err != nil {
		return 10
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), fakeSpiderName+".json"))
	var f fakeSpiderFixture
	if err != nil || json.Unmarshal(raw, &f) != nil {
		return 10
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println("spider_cli " + f.Version)
		return 0
	}
	if os.Getenv("TB_CATALYST_API_KEY") != "" || os.Getenv("HTTPS_PROXY") != "" {
		return 11
	}
	if len(args) != 17 || args[0] != "--url" || args[3] != "--http" || args[5] != "--limit" || args[6] != f.Limit || args[15] != "scrape" || args[16] != "--output-html" {
		return 15
	}
	if f.Calls != "" {
		calls, err := os.OpenFile(f.Calls, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return 12
		}
		_, err = fmt.Fprintln(calls, args[1])
		if closeErr := calls.Close(); err != nil || closeErr != nil {
			return 12
		}
	}
	page, ok := f.Pages[args[1]]
	if !ok {
		return 17
	}
	fmt.Println(page)
	return 0
}

// fakeSpider copies the running test binary under a name that TestMain
// recognises and writes its fixture next to it.
func fakeSpider(t *testing.T, f fakeSpiderFixture) string {
	t.Helper()
	self, err := os.Executable()
	require.NoError(t, err)
	data, err := os.ReadFile(self)
	require.NoError(t, err)
	dir := t.TempDir()
	binary := filepath.Join(dir, fakeSpiderName+filepath.Ext(self))
	require.NoError(t, os.WriteFile(binary, data, 0700))
	raw, err := json.Marshal(f)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, fakeSpiderName+".json"), raw, 0600))
	return binary
}

func testSpiderPage() spiderPage {
	return spiderPage{URL: testPolicy().Sources[0].URL, StatusCode: 200,
		Headers: map[string]string{"content-type": "text/html; charset=utf-8"}, Content: "<main>Timing is TBA.</main>"}
}

func TestSpiderEvidenceStableAndReadable(t *testing.T) {
	seed := testSpiderPage()
	article := seed
	article.URL += "/article"
	source := testPolicy().Sources[0]
	first, status, err := spiderEvidence(source, spiderJSON(t, seed, article), fixtureNow)
	require.NoError(t, err)
	require.Equal(t, 200, status)
	second, _, err := spiderEvidence(source, spiderJSON(t, article, seed), fixtureNow)
	require.NoError(t, err)
	require.Equal(t, first, second, "arrival order must not create a revision")
	require.Equal(t, spiderExtractionVersion, first.ExtractionVersion)
	require.Len(t, first.Documents, 2)
	a := Archive{Dir: t.TempDir()}
	id, err := a.Put("evidence", first)
	require.NoError(t, err)
	var restored Evidence
	require.NoError(t, a.Read("evidence", id, &restored))
	require.Equal(t, first, restored)
}

func TestSpiderRejectsInvalidOrIncompleteOutput(t *testing.T) {
	seed := testSpiderPage()
	for name, mutate := range map[string]func(*spiderPage){
		"cross origin":            func(p *spiderPage) { p.URL = "https://other.example/article" },
		"query":                   func(p *spiderPage) { p.URL += "?key=secret" },
		"wrong scheme":            func(p *spiderPage) { p.URL = "http://example.org/" },
		"missing seed":            func(p *spiderPage) { p.URL += "/other" },
		"missing status feature":  func(p *spiderPage) { p.StatusCode = 0 },
		"rate limited":            func(p *spiderPage) { p.StatusCode = 429 },
		"missing headers feature": func(p *spiderPage) { p.Headers = nil },
		"empty body":              func(p *spiderPage) { p.Content = "<script>only script</script>" },
	} {
		t.Run(name, func(t *testing.T) {
			page := testSpiderPage()
			mutate(&page)
			_, _, err := spiderEvidence(testPolicy().Sources[0], spiderJSON(t, page), fixtureNow)
			require.Error(t, err)
		})
	}
	tooMany := []spiderPage{seed}
	for n := 1; n <= spiderPages; n++ {
		page := seed
		page.URL += fmt.Sprint(n)
		tooMany = append(tooMany, page)
	}
	for name, raw := range map[string][]byte{
		"empty":           nil,
		"duplicate":       spiderJSON(t, seed, seed),
		"budget":          spiderJSON(t, tooMany...),
		"trailing":        append(spiderJSON(t, seed), 'x'),
		"unknown feature": bytes.Replace(spiderJSON(t, seed), []byte(`"status_code":`), []byte(`"unexpected":1,"status_code":`), 1),
		"duplicate key":   bytes.Replace(spiderJSON(t, seed), []byte(`"status_code":`), []byte(`"status_code":201,"status_code":`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := spiderEvidence(testPolicy().Sources[0], raw, fixtureNow)
			require.Error(t, err)
		})
	}
}

func TestSpiderArchiveExpansionAndOutputLimits(t *testing.T) {
	var pages []spiderPage
	for n := range spiderPages {
		p := testSpiderPage()
		if n > 0 {
			p.URL += fmt.Sprint(n)
		}
		p.Content = "<main>Valid text.</main><!--" + strings.Repeat("\t", 700000) + "-->"
		pages = append(pages, p)
	}
	// JSON inside Raw is escaped a second time in the archive record.
	raw := spiderJSON(t, pages...)
	require.Less(t, len(raw), spiderOutputBytes)
	_, _, err := spiderEvidence(testPolicy().Sources[0], raw, fixtureNow)
	require.ErrorContains(t, err, "archive limit")
	a := Archive{Dir: t.TempDir()}
	_, err = a.Put("records", strings.Repeat("x", maxArchiveBytes))
	require.ErrorContains(t, err, "size limit")
	ids, err := a.IDs("records")
	require.NoError(t, err)
	require.Empty(t, ids, "oversized records must never be accepted")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	bounded := boundedOutput{cancel: cancel}
	_, err = bounded.Write(make([]byte, spiderOutputBytes+1))
	require.Error(t, err)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Zero(t, bounded.Len())
}

func TestRejectedArchiveRecordInvalidatesPreviousSnapshot(t *testing.T) {
	p := testPolicy()
	a := Archive{Dir: t.TempDir()}
	c, err := NewCollector(p, a)
	require.NoError(t, err)
	c.minInterval = 0
	body := "first snapshot"
	c.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/robots.txt" {
			return response(404, "text/plain", ""), nil
		}
		return response(200, "text/plain", body), nil
	})}
	_, err = c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	body = strings.Repeat("\x00", 500000)
	_, err = c.Collect(t.Context(), p.Sources[0])
	require.ErrorContains(t, err, "archive record exceeds")
	_, err = a.LatestEvidence(p)
	require.ErrorContains(t, err, "latest fetch failed")
	ids, err := a.IDs("evidence")
	require.NoError(t, err)
	require.Len(t, ids, 1, "retain the old snapshot without presenting it as current")
}

func TestSpiderSubprocessAndCache(t *testing.T) {
	// A fake binary exercises argument/env handling without a public crawl.
	dir := t.TempDir()
	binary := fakeSpider(t, fakeSpiderFixture{Version: SpiderVersion, Limit: "5",
		Pages: map[string]string{testSpiderPage().URL: string(spiderJSON(t, testSpiderPage()))}})
	t.Setenv("TB_CATALYST_API_KEY", "fixture-secret")
	t.Setenv("HTTPS_PROXY", "http://must-not-be-inherited.invalid")
	p := testPolicy()
	c, err := NewSpiderCollector(p, Archive{Dir: filepath.Join(dir, "archive")}, binary)
	require.NoError(t, err)
	var robotsCalls int
	c.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "/robots.txt", r.URL.Path)
		robotsCalls++
		return response(404, "text/plain", ""), nil
	})}
	first, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.NoError(t, os.Remove(binary))
	cached, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.Equal(t, first, cached)
	require.Equal(t, 1, robotsCalls, "cache must avoid another robots/subprocess request")
	// An old HTTP extraction cannot satisfy the Spider cache.
	c.spiderBinary = ""
	_, err = c.Collect(t.Context(), p.Sources[0])
	require.ErrorContains(t, err, "cached source evidence missing")
}

func TestSpiderRefusesMissingOrWrongBinary(t *testing.T) {
	for _, binary := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		_, err := NewSpiderCollector(testPolicy(), Archive{Dir: t.TempDir()}, binary)
		require.Error(t, err)
	}
	for _, version := range []string{"0.0.0", "2.53.9"} {
		t.Run(version, func(t *testing.T) {
			binary := fakeSpider(t, fakeSpiderFixture{Version: version})
			c, err := NewSpiderCollector(testPolicy(), Archive{Dir: t.TempDir()}, binary)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			_, _, err = c.fetchSpider(ctx, testPolicy().Sources[0])
			require.ErrorContains(t, err, "version must be")
		})
	}
}

func TestSpiderArticleLinksSeedOrderSameHost(t *testing.T) {
	source := testPolicy().Sources[0]
	source.ArticlePattern = `^/\d{4}/\d{2}/\d{2}/[a-z0-9-]+$`
	seed := testSpiderPage()
	seed.Content = `<a href="/2026/10/05/old">a</a><a href="/category/x">c</a>
<a href="https://evil.example/2026/10/30/x">e</a><a href="http://example.org/2026/10/29/plain">p</a>
<a href="/2026/10/22/new?utm=1#top">n</a><a href="/2026/10/22/new">dup</a>
<a href="/2026/09/01/a">1</a><a href="/2026/09/02/b">2</a><a href="/2026/09/03/c">3</a>`
	links, err := articleLinks(source, spiderJSON(t, seed))
	require.NoError(t, err)
	require.Equal(t, []string{"https://example.org/2026/10/05/old", "https://example.org/2026/10/22/new",
		"https://example.org/2026/09/01/a", "https://example.org/2026/09/02/b"}, links)
	seed.Content = `<a href="/category/x">c</a>`
	_, err = articleLinks(source, spiderJSON(t, seed))
	require.ErrorContains(t, err, "no article links")
}

func TestSpiderScrapesSelectedArticles(t *testing.T) {
	dir := t.TempDir()
	seed := testSpiderPage()
	seed.Content = `<a href="/2026/10/22/new">n</a><a href="/category/x">c</a>`
	article := testSpiderPage()
	article.URL, article.Content = "https://example.org/2026/10/22/new", "<main>Upgrade article</main>"
	binary := fakeSpider(t, fakeSpiderFixture{Version: SpiderVersion, Limit: "1", Pages: map[string]string{
		"https://example.org/events":         string(spiderJSON(t, seed)),
		"https://example.org/2026/10/22/new": string(spiderJSON(t, article)),
	}})
	p := testPolicy()
	p.Sources[0].ArticlePattern = `^/\d{4}/\d{2}/\d{2}/[a-z0-9-]+$`
	require.NoError(t, p.Validate())
	c, err := NewSpiderCollector(p, Archive{Dir: filepath.Join(dir, "archive")}, binary)
	require.NoError(t, err)
	c.client = &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return response(404, "text/plain", ""), nil
	})}
	e, err := c.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.Contains(t, e.Body, "Fetched page: https://example.org/2026/10/22/new")
	require.Contains(t, e.Body, "Upgrade article")
	p.Sources[0].ArticlePattern = "("
	require.ErrorContains(t, p.Validate(), "article pattern")
}

func TestSpiderLinkedScrapesOtherHostPagesNewestFirst(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	lead := Source{ID: "lead", URL: "https://t.example/s/channel"}
	source := Source{ID: "sale", URL: "https://example.org/list", ArticlePattern: `^/sale/[a-z]+$`}
	leadPage, old, fresh := testSpiderPage(), testSpiderPage(), testSpiderPage()
	leadPage.URL = lead.URL
	leadPage.Content = `<a href="https://example.org/sale/old">o</a><a href="https://evil.example/sale/x">e</a>
<a href="https://example.org/other">x</a><a href="/sale/relative">r</a><a href="https://example.org/sale/new?ref=1">n</a>`
	old.URL, old.Content = "https://example.org/sale/old", "<main>Old sale</main>"
	fresh.URL, fresh.Content = "https://example.org/sale/new", "<main>New sale</main>"
	binary := fakeSpider(t, fakeSpiderFixture{Version: SpiderVersion, Limit: "1", Calls: calls, Pages: map[string]string{
		"https://t.example/s/channel":  string(spiderJSON(t, leadPage)),
		"https://example.org/sale/old": string(spiderJSON(t, old)),
		"https://example.org/sale/new": string(spiderJSON(t, fresh)),
	}})
	c := &Collector{spiderBinary: binary, client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		return response(404, "text/plain", ""), nil
	})}}
	leadEvidence, e, err := c.fetchLinked(t.Context(), lead, source)
	require.NoError(t, err)
	require.Equal(t, "lead", leadEvidence.SourceID)
	require.Len(t, leadEvidence.Documents, 1)
	require.Equal(t, "sale", e.SourceID)
	require.Len(t, e.Documents, 2, "only pages on the source host matching the pattern")
	require.Contains(t, e.Body, "New sale")
	require.Contains(t, e.Body, "Old sale")
	order, err := os.ReadFile(calls)
	require.NoError(t, err)
	require.Equal(t, "https://t.example/s/channel\nhttps://example.org/sale/new\nhttps://example.org/sale/old\n", string(order))

	source.ArticlePattern = `^/nothing$`
	_, _, err = c.fetchLinked(t.Context(), lead, source)
	require.ErrorContains(t, err, "no article links")
}
