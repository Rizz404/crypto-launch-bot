package catalyst

import (
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const ExtractionVersion = "documents-v2"
const maxFeedDocuments = 200

// Document identity tracks a publisher URL, not a verified asset or event.
type Document struct {
	ID            string    `json:"id"`
	Revision      string    `json:"revision"`
	URL           string    `json:"url"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	PublishedText string    `json:"published_text"`
	UpdatedText   string    `json:"updated_text"`
	Body          string    `json:"body"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
}

func extractDocuments(source Source, media, raw string, now time.Time) ([]Document, string, error) {
	if len(raw) > MaxDocumentBytes || !utf8.ValidString(raw) {
		return nil, "", errors.New("document exceeds limit or is not UTF-8")
	}
	var docs []Document
	var err error
	switch media {
	case "application/rss+xml", "application/atom+xml", "application/xml", "text/xml":
		docs, err = extractFeed(source.URL, raw)
	case "text/html":
		var d Document
		d, err = extractHTML(raw)
		d.URL, d.Kind = source.URL, "page"
		docs = []Document{d}
	default:
		docs = []Document{{URL: source.URL, Kind: "document", Body: strings.TrimSpace(raw)}}
	}
	if err != nil {
		return nil, "", err
	}
	var text strings.Builder
	seen := map[string]bool{}
	for n := range docs {
		d := &docs[n]
		if strings.TrimSpace(d.Body) == "" {
			return nil, "", errors.New("source has no extractable document body")
		}
		d.ID = Hash([]string{source.ID, d.URL})
		if seen[d.ID] {
			return nil, "", errors.New("feed contains duplicate document URLs")
		}
		seen[d.ID] = true
		d.Revision = Hash(*d)
		d.FirstSeenAt = now
		if d.Title != "" {
			text.WriteString("Title: " + d.Title + "\n")
		}
		if d.PublishedText != "" {
			text.WriteString("Published (publisher claim, not event time): " + d.PublishedText + "\n")
		}
		if d.UpdatedText != "" {
			text.WriteString("Updated (publisher claim): " + d.UpdatedText + "\n")
		}
		if d.Kind == "feed_entry" {
			text.WriteString("Article URL: " + d.URL + "\n")
		}
		text.WriteString(d.Body + "\n\n")
	}
	if text.Len() > MaxDocumentBytes {
		return nil, "", errors.New("extracted source exceeds text limit")
	}
	return docs, strings.TrimSpace(text.String()), nil
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func visibleText(n *html.Node) string {
	var out strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "template", "nav", "footer", "head", "iframe", "svg", "canvas", "form":
				return
			}
			for _, a := range n.Attr {
				if a.Key == "hidden" || (a.Key == "aria-hidden" && a.Val == "true") {
					return
				}
			}
		}
		switch n.Type {
		case html.TextNode:
			out.WriteString(n.Data)
		case html.ElementNode:
			switch n.Data {
			case "p", "div", "section", "article", "li", "br", "h1", "h2", "h3", "tr":
				out.WriteByte(' ')
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "div", "section", "article", "li", "h1", "h2", "h3", "td", "tr":
				out.WriteByte(' ')
			}
		}
	}
	walk(n)
	return strings.Join(strings.Fields(out.String()), " ")
}

var ldPublished = regexp.MustCompile(`"datePublished"\s*:\s*"([^"]{1,64})"`)

func extractHTML(raw string) (Document, error) {
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return Document{}, errors.New("invalid source HTML")
	}
	d := Document{}
	var mainNode, body *html.Node
	var timeText string
	for n := range root.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "main":
			if mainNode == nil {
				mainNode = n
			}
		case "body":
			body = n
		case "title":
			d.Title = visibleText(n)
		case "meta":
			key := strings.ToLower(attr(n, "property"))
			if key == "" {
				key = strings.ToLower(attr(n, "name"))
			}
			if key == "" {
				key = strings.ToLower(attr(n, "itemprop"))
			}
			switch key {
			case "article:published_time", "datepublished":
				d.PublishedText = strings.TrimSpace(attr(n, "content"))
			case "article:modified_time", "datemodified":
				d.UpdatedText = strings.TrimSpace(attr(n, "content"))
			}
		case "script":
			if strings.EqualFold(strings.TrimSpace(attr(n, "type")), "application/ld+json") && n.FirstChild != nil && d.PublishedText == "" {
				if m := ldPublished.FindStringSubmatch(n.FirstChild.Data); m != nil {
					d.PublishedText = m[1]
				}
			}
		case "time":
			if timeText == "" {
				timeText = strings.TrimSpace(attr(n, "datetime"))
			}
		}
	}
	// The first <time> is only a fallback; structured metadata names the publish date explicitly.
	if d.PublishedText == "" {
		d.PublishedText = timeText
	}
	if mainNode != nil {
		body = mainNode
	}
	if body == nil {
		return d, errors.New("source HTML has no body")
	}
	d.Body = visibleText(body)
	return d, nil
}

type feedEntry struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Encoded     string `xml:"encoded"`
	Content     struct {
		Type  string `xml:"type,attr"`
		Text  string `xml:",chardata"`
		Inner string `xml:",innerxml"`
	} `xml:"content"`
	Summary   string `xml:"summary"`
	PubDate   string `xml:"pubDate"`
	Date      string `xml:"date"`
	Published string `xml:"published"`
	Updated   string `xml:"updated"`
	Links     []struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
		Text string `xml:",chardata"`
	} `xml:"link"`
}

func extractFeed(baseURL, raw string) ([]Document, error) {
	dec := xml.NewDecoder(strings.NewReader(raw))
	var root string
	var depth int
	var docs []Document
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("invalid source XML")
		}
		switch t := token.(type) {
		case xml.Directive:
			return nil, errors.New("XML directives are not supported")
		case xml.StartElement:
			if depth == 0 {
				if root != "" || (t.Name.Local != "rss" && t.Name.Local != "feed") {
					return nil, errors.New("source XML is not RSS or Atom")
				}
				root = t.Name.Local
			}
			isEntry := (root == "rss" && t.Name.Local == "item" && depth == 2) || (root == "feed" && t.Name.Local == "entry" && depth == 1)
			if !isEntry {
				depth++
				if depth > 64 {
					return nil, errors.New("source XML nesting exceeds limit")
				}
				continue
			}
			if len(docs) >= maxFeedDocuments {
				return nil, errors.New("feed exceeds document limit")
			}
			var entry feedEntry
			if err := dec.DecodeElement(&entry, &t); err != nil {
				return nil, errors.New("invalid RSS/Atom entry")
			}
			d, err := entry.document(baseURL)
			if err != nil {
				return nil, err
			}
			docs = append(docs, d)
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return nil, errors.New("trailing source XML content")
			}
		}
	}
	if len(docs) == 0 {
		return nil, errors.New("feed has no entries")
	}
	return docs, nil
}

func (e feedEntry) document(baseURL string) (Document, error) {
	base, _ := url.Parse(baseURL)
	var link string
	for _, l := range e.Links {
		if l.Rel == "" || l.Rel == "alternate" {
			link = l.Href
			if link == "" {
				link = strings.TrimSpace(l.Text)
			}
			if link != "" {
				break
			}
		}
	}
	u, err := url.Parse(link)
	if err != nil || link == "" {
		return Document{}, errors.New("feed entry needs an article URL")
	}
	u = base.ResolveReference(u)
	if _, err = publicURL(u.String()); err != nil || !strings.EqualFold(u.Hostname(), base.Hostname()) {
		return Document{}, errors.New("feed article URL must stay on its configured HTTPS origin without query or fragment")
	}
	body := e.Encoded
	if body == "" {
		body = e.Content.Text
		if e.Content.Type == "xhtml" {
			body = e.Content.Inner
		}
	}
	if body == "" {
		body = e.Description
	}
	if body == "" {
		body = e.Summary
	}
	clean, err := extractHTML(body)
	if err != nil {
		return Document{}, err
	}
	published := e.Published
	if published == "" {
		published = e.PubDate
	}
	if published == "" {
		published = e.Date
	}
	return Document{URL: u.String(), Kind: "feed_entry", Title: strings.Join(strings.Fields(e.Title), " "), PublishedText: strings.TrimSpace(published), UpdatedText: strings.TrimSpace(e.Updated), Body: clean.Body}, nil
}
