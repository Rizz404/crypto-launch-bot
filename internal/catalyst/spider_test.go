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
	// A shell fixture exercises argument/env handling without a public crawl.
	dir := t.TempDir()
	binary := filepath.Join(dir, "spider")
	payload := string(spiderJSON(t, testSpiderPage()))
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  printf 'spider_cli 2.53.9-p3.1\n'
  exit 0
fi
[ -z "$TB_CATALYST_API_KEY" ] || exit 11
[ -z "$HTTPS_PROXY" ] || exit 12
[ "$1" = "--url" ] || exit 13
[ "$4" = "--http" ] || exit 14
[ "$6" = "--limit" ] && [ "$7" = "5" ] || exit 15
[ "${16}" = "scrape" ] && [ "${17}" = "--output-html" ] || exit 16
printf '%s\n' '` + payload + `'
`
	require.NoError(t, os.WriteFile(binary, []byte(script), 0700))
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
			binary := filepath.Join(t.TempDir(), "spider")
			require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nprintf 'spider_cli "+version+"\\n'\n"), 0700))
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
	binary := filepath.Join(dir, "spider")
	seed := testSpiderPage()
	seed.Content = `<a href="/2026/10/22/new">n</a><a href="/category/x">c</a>`
	article := testSpiderPage()
	article.URL, article.Content = "https://example.org/2026/10/22/new", "<main>Upgrade article</main>"
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  printf 'spider_cli 2.53.9-p3.1\n'
  exit 0
fi
[ "$7" = "1" ] || exit 15
case "$2" in
  https://example.org/events) printf '%s\n' '` + string(spiderJSON(t, seed)) + `' ;;
  https://example.org/2026/10/22/new) printf '%s\n' '` + string(spiderJSON(t, article)) + `' ;;
  *) exit 17 ;;
esac
`
	require.NoError(t, os.WriteFile(binary, []byte(script), 0700))
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
