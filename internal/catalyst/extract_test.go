package catalyst

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func rss(body string) string {
	return `<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel><title>Publisher</title><item><title>Upgrade research</title><link>https://example.org/article</link><pubDate>Mon, 26 Oct 2026 09:00:00 GMT</pubDate><description>Summary only</description><content:encoded><![CDATA[` + body + `]]></content:encoded></item></channel></rss>`
}

func TestExtractFeedAndHTML(t *testing.T) {
	source := testPolicy().Sources[0]
	docs, text, err := extractDocuments(source, "application/rss+xml", rss(`<p>Activation date is <b>TBA</b>.</p><script>ignore schema</script>`), fixtureNow)
	require.NoError(t, err)
	require.Len(t, docs, 1)
	require.Equal(t, "Activation date is TBA.", docs[0].Body)
	require.Equal(t, "https://example.org/article", docs[0].URL)
	require.Equal(t, "feed_entry", docs[0].Kind)
	require.Contains(t, text, "not event time")
	require.NotContains(t, text, "ignore schema")
	require.NotContains(t, text, "Summary only")
	require.Equal(t, fixtureNow, docs[0].FirstSeenAt)

	atom := `<feed xmlns="http://www.w3.org/2005/Atom"><entry><title>New result</title><link rel="self" href="https://example.org/feed"/><link rel="alternate" href="/article"/><published>2026-10-26T00:00:00Z</published><updated>2026-10-26T01:00:00Z</updated><content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml"><p>Timing remains TBA</p></div></content></entry></feed>`
	docs, _, err = extractDocuments(source, "application/atom+xml", atom, fixtureNow)
	require.NoError(t, err)
	require.Equal(t, "Timing remains TBA", docs[0].Body)
	require.Equal(t, "2026-10-26T01:00:00Z", docs[0].UpdatedText)

	page := `<html><head><title>Official &amp; uncertain</title><meta property="article:published_time" content="2026-10-26T00:00:00Z"></head><body><nav>menu</nav><main><h1>Research</h1><p>alpha<b>beta</b> &amp; no event date.</p><span hidden>hidden</span><div aria-hidden="true">also hidden</div><script>malicious()</script><style>.x{}</style><form>password</form><iframe>nested</iframe></main><footer>footer</footer></body></html>`
	docs, text, err = extractDocuments(source, "text/html", page, fixtureNow)
	require.NoError(t, err)
	require.Equal(t, "Official & uncertain", docs[0].Title)
	require.Equal(t, "page", docs[0].Kind)
	require.Equal(t, "Research alphabeta & no event date.", docs[0].Body)
	for _, excluded := range []string{"menu", "hidden", "malicious", "password", "nested", "footer", ".x"} {
		require.NotContains(t, text, excluded)
	}
}

func TestExtractRejectsAmbiguousOrUnsafeFeeds(t *testing.T) {
	source := testPolicy().Sources[0]
	for name, raw := range map[string]string{
		"wrong root":    `<config><item>secret</item></config>`,
		"empty feed":    `<rss><channel/></rss>`,
		"malformed":     `<rss><channel><item></rss>`,
		"directive":     `<!DOCTYPE rss SYSTEM "https://example.org/secret">` + rss("body"),
		"external link": strings.Replace(rss("body"), "https://example.org/article", "https://other.example/article", 1),
		"query":         strings.Replace(rss("body"), "/article</link>", "/article?key=secret</link>", 1),
		"private link":  strings.Replace(rss("body"), "https://example.org/article", "http://127.0.0.1/secret", 1),
		"empty body":    rss(`<script>only script</script>`),
		"duplicate URL": strings.Replace(rss("body"), "</channel>", `<item><link>https://example.org/article</link><description>duplicate</description></item></channel>`, 1),
		"trailing XML":  rss("body") + `<feed/>`,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := extractDocuments(source, "application/rss+xml", raw, fixtureNow)
			require.Error(t, err)
		})
	}
	var items strings.Builder
	for n := 0; n <= maxFeedDocuments; n++ {
		fmt.Fprintf(&items, "<item><link>https://example.org/%d</link><description>body</description></item>", n)
	}
	_, _, err := extractDocuments(source, "text/xml", "<rss><channel>"+items.String()+"</channel></rss>", fixtureNow)
	require.Error(t, err)
	_, _, err = extractDocuments(source, "text/plain", string([]byte{0xff}), fixtureNow)
	require.Error(t, err)
}

func TestDocumentRevisionSurvivesFeedReorderingAndReversion(t *testing.T) {
	p := testPolicy()
	archive := Archive{Dir: t.TempDir()}
	collector, err := NewCollector(p, archive)
	require.NoError(t, err)
	collector.minInterval = 0
	body := rss("Original report")
	collector.client = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/robots.txt" {
			return response(404, "text/plain", ""), nil
		}
		return response(200, "application/rss+xml", body), nil
	})}
	first, err := collector.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	body = strings.Replace(body, "<title>Publisher</title>", "<title>Publisher changed</title>", 1)
	second, err := collector.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, first.Documents, second.Documents, "feed chrome does not change document identity or receipt time")
	body = rss("Official correction: withdrawn; no replacement date")
	third, err := collector.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.Equal(t, first.Documents[0].ID, third.Documents[0].ID)
	require.NotEqual(t, first.Documents[0].Revision, third.Documents[0].Revision)
	body = rss("Original report")
	last, err := collector.Collect(t.Context(), p.Sources[0])
	require.NoError(t, err)
	require.Equal(t, first.ID, last.ID)
	require.Equal(t, first.Documents, last.Documents)
	report, err := archive.Report(p)
	require.NoError(t, err)
	require.Len(t, report.Documents, 1)
	require.Equal(t, first.Documents[0].Revision, report.Documents[0].Revision)
	require.Equal(t, first.ID, report.Documents[0].EvidenceID)
	ids, err := archive.IDs("evidence")
	require.NoError(t, err)
	require.Len(t, ids, 3, "all content revisions remain immutable")
}

func TestAnalystValidatesOnlyVisiblePrefix(t *testing.T) {
	for _, quoteVisible := range []bool{true, false} {
		t.Run(fmt.Sprint(quoteVisible), func(t *testing.T) {
			now := time.Now().UTC()
			input := testInput(now)
			body := "Visible TBA. " + strings.Repeat("x", 11984) + "日本語" + " Hidden suffix."
			input.Evidence[0].Body, input.Evidence[0].Raw = body, "raw must not reach model"
			input.Evidence[0].Documents = []Document{{Body: "metadata body must not reach model"}}
			analysis := Analysis{Action: "WATCH", Symbol: "ETHUSDT", Reason: "no verified date", Citations: []Citation{{EvidenceID: input.Evidence[0].ID, Quote: "Visible TBA."}}}
			if !quoteVisible {
				analysis.Citations[0].Quote = "Hidden suffix."
			}
			analyst, err := NewAnalyst("https://example.org/v1", "fixture", "fixture-key")
			require.NoError(t, err)
			analyst.client = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
				var request struct{ Messages []struct{ Content string } }
				require.NoError(t, json.NewDecoder(req.Body).Decode(&request))
				var packet struct{ Evidence []packetEvidence }
				require.NoError(t, json.Unmarshal([]byte(request.Messages[1].Content), &packet))
				require.Len(t, packet.Evidence, 1)
				e := packet.Evidence[0]
				require.True(t, e.BodyTruncated)
				require.Equal(t, len(body), e.OriginalBodyBytes)
				require.True(t, utf8.ValidString(e.Body))
				require.LessOrEqual(t, len(e.Body), 12000)
				require.Empty(t, e.Raw)
				require.Empty(t, e.Documents)
				require.NotContains(t, e.Body, "Hidden suffix.")
				raw, _ := json.Marshal(analysis)
				envelope, _ := json.Marshal(map[string]any{"model": "served-fixture", "choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]string{"content": string(raw)}}}})
				return response(200, "application/json", string(envelope)), nil
			})}
			review, err := analyst.Analyze(t.Context(), Archive{Dir: t.TempDir()}, input.Policy, input.Evidence, now)
			require.Equal(t, quoteVisible, review.Valid)
			if quoteVisible {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "citation does not match")
			}
		})
	}
}
