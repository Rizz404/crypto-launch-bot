package catalyst

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Run explicitly in the dedicated image with --network none; only loopback
// fixtures are needed. Unit tests do not require an installed Spider binary.
func TestSpiderBinaryProtocol(t *testing.T) {
	binary := os.Getenv("TB_TEST_SPIDER_BINARY")
	if binary == "" {
		t.Skip("requires explicit pinned Spider binary in an isolated container")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	check := exec.CommandContext(ctx, binary, "--version")
	check.Env = []string{"RUST_LOG=error"}
	version, err := check.Output()
	require.NoError(t, err)
	require.Equal(t, "spider_cli "+SpiderVersion, strings.TrimSpace(string(version)))

	var requests, tunnels atomic.Int32
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		t.Logf("fixture request: %s", r.URL.Path)
		if r.URL.Path == "/robots.txt" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.URL.Path == "/events" {
			_, _ = w.Write([]byte(`<main>Fixture seed. <a href="/article">Article</a></main>`))
			return
		}
		_, _ = w.Write([]byte("<main>Fixture article. Date remains TBA.</main>"))
	}))
	defer upstream.Close()
	proxy := httptest.NewServer(spiderProxyHandler(ctx, "example.org", func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr != "example.org:443" {
			return nil, errors.New("unexpected fixture destination")
		}
		tunnels.Add(1)
		var dial net.Dialer
		return dial.DialContext(ctx, network, upstream.Listener.Addr().String())
	}))
	defer proxy.Close()
	source := testPolicy().Sources[0]
	// Only this fixture invocation accepts its self-signed certificate; the
	// production collector never passes --accept-invalid-certs.
	cmd := exec.CommandContext(ctx, binary, "--url", source.URL,
		"--respect-robots-txt", "--http", "--return-headers", "--limit", "5",
		"--depth", "2", "--delay", "1000", "--agent", collectorAgent+"/1.0",
		"--proxy-url", proxy.URL, "--accept-invalid-certs", "scrape", "--output-html")
	cmd.Env = []string{"RUST_LOG=error"}
	cmd.WaitDelay = time.Second
	output := &boundedOutput{cancel: cancel}
	cmd.Stdout = output
	require.NoError(t, cmd.Run())
	evidence, status, err := spiderEvidence(source, output.Bytes(), time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, evidence.Documents, 2, "pinned binary must follow the fixture article link")
	require.Contains(t, evidence.Body, "Date remains TBA.")
	require.Positive(t, tunnels.Load(), "requests must reach the configured CONNECT proxy")
	require.GreaterOrEqual(t, requests.Load(), int32(3), "robots plus both pages")
	archive := Archive{Dir: t.TempDir()}
	id, err := archive.Put("evidence", evidence)
	require.NoError(t, err)
	var restored Evidence
	require.NoError(t, archive.Read("evidence", id, &restored))
	require.Equal(t, evidence, restored)
}
