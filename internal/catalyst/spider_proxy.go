package catalyst

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Spider's requests use an ephemeral loopback CONNECT proxy so redirects and
// discovered links get the same DNS/IP checks as the Go collector before dial.
func spiderProxy(ctx context.Context, source string) (string, func(), error) {
	host, err := sourceHost(source)
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		cancel()
		return "", nil, errors.New("cannot start private Spider proxy")
	}
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 8192, Handler: spiderProxyHandler(ctx, host, publicDial)}
	go func() { _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String(), func() { cancel(); _ = server.Close() }, nil
}

func spiderProxyHandler(ctx context.Context, allowedHost string, dial func(context.Context, string, string) (net.Conn, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, port, err := net.SplitHostPort(r.Host)
		if r.Method != http.MethodConnect || err != nil || port != "443" || !strings.EqualFold(host, allowedHost) {
			http.Error(w, "Spider target denied", http.StatusForbidden)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Spider tunnel unavailable", http.StatusInternalServerError)
			return
		}
		upstream, err := dial(ctx, "tcp", net.JoinHostPort(host, port))
		if err != nil {
			http.Error(w, "Spider target unavailable", http.StatusBadGateway)
			return
		}
		defer func() { _ = upstream.Close() }()
		conn, buffered, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		stop := context.AfterFunc(ctx, func() { _ = upstream.Close(); _ = conn.Close() })
		defer stop()
		if _, err = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			return
		}
		if err = buffered.Flush(); err != nil {
			return
		}
		done := make(chan struct{})
		go func() {
			_, _ = io.Copy(upstream, buffered)
			_ = upstream.Close()
			close(done)
		}()
		_, _ = io.Copy(conn, upstream)
		_ = conn.Close()
		<-done
	})
}
