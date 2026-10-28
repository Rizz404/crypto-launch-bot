package catalyst

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSpiderProxyDeniesBeforeDial(t *testing.T) {
	dial := func(context.Context, string, string) (net.Conn, error) {
		t.Error("denied target must not be dialed")
		return nil, errors.New("unexpected dial")
	}
	h := spiderProxyHandler(t.Context(), "example.org", dial)
	for _, target := range []string{"other.example:443", "example.org:80", "127.0.0.1:443", "[::1]:443", "example.org", "example.org:0443"} {
		r := httptest.NewRequest(http.MethodConnect, "http://"+target, nil)
		r.Host = target
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		require.Equal(t, http.StatusForbidden, w.Code, target)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://example.org:443/", nil))
	require.Equal(t, http.StatusForbidden, w.Code)
	w = httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodConnect, "http://example.org:443", nil)
	r.Host = "example.org:443"
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusInternalServerError, w.Code, "unsupported hijacker must fail without panic or dial")
}

func TestSpiderProxyRejectsNonPublicDNSAndDialFailure(t *testing.T) {
	// Numeric loopback is resolved locally; the guard must reject it before dial.
	_, err := publicDial(t.Context(), "tcp", "127.0.0.1:443")
	require.ErrorContains(t, err, "non-public")
	h := spiderProxyHandler(t.Context(), "example.org", func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("fixture DNS failure")
	})
	s := httptest.NewServer(h)
	defer s.Close()
	conn, err := net.DialTimeout("tcp", s.Listener.Addr().String(), time.Second)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetDeadline(time.Now().Add(3*time.Second)))
	_, err = io.WriteString(conn, "CONNECT example.org:443 HTTP/1.1\r\nHost: example.org:443\r\n\r\n")
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestSpiderProxyRelaysBufferedBytesAndCancelsTunnel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	upstream, peer := net.Pipe()
	defer func() { _ = peer.Close(); _ = upstream.Close() }()
	finished := make(chan struct{})
	h := spiderProxyHandler(ctx, "example.org", func(_ context.Context, network, addr string) (net.Conn, error) {
		if network != "tcp" || addr != "example.org:443" {
			return nil, errors.New("unexpected destination")
		}
		return upstream, nil
	})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		h.ServeHTTP(w, r)
	}))
	defer s.Close()
	conn, err := net.DialTimeout("tcp", s.Listener.Addr().String(), time.Second)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetDeadline(time.Now().Add(3*time.Second)))
	require.NoError(t, peer.SetDeadline(time.Now().Add(3*time.Second)))
	_, err = io.WriteString(conn, "CONNECT example.org:443 HTTP/1.1\r\nHost: example.org:443\r\n\r\nearly bytes")
	require.NoError(t, err)
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data := make([]byte, len("early bytes"))
	_, err = io.ReadFull(peer, data)
	require.NoError(t, err)
	require.Equal(t, "early bytes", string(data))
	_, err = io.WriteString(peer, "reply")
	require.NoError(t, err)
	data = make([]byte, len("reply"))
	_, err = io.ReadFull(reader, data)
	require.NoError(t, err)
	require.Equal(t, "reply", string(data))
	cancel()
	_, err = reader.ReadByte()
	require.ErrorIs(t, err, io.EOF)
	_, err = peer.Read(data)
	require.ErrorIs(t, err, io.EOF)
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("proxy relay did not finish after cancellation")
	}
}
