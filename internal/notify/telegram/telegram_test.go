package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func newBot() *Bot {
	return New("fake-token", []int64{42}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestCommandsEscapeAndQueueBound(t *testing.T) {
	b := newBot()
	b.Handle("status", "<status>", func(_ context.Context, args []string) string { return strings.Join(args, " ") })
	assert.Equal(t, "one two", b.dispatch(context.Background(), "/status@MyBot one two"))
	assert.Contains(t, b.dispatch(context.Background(), "/unknown"), "&lt;status&gt;")
	assert.Equal(t, "&lt;b&gt;&amp;", Escape("<b>&"))
	for range queueSize + 1 {
		b.Notify("test")
	}
	assert.Len(t, b.queue, queueSize)
}

func TestTransportErrorsDoNotExposeToken(t *testing.T) {
	b := newBot()
	b.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return nil, &url.Error{Op: "Post", URL: r.URL.String(), Err: errors.New("offline")}
	})}
	err := b.send(context.Background(), 42, "test")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "fake-token")
	assert.Contains(t, err.Error(), "offline")
}

// The shutdown notice is queued right before the bot cancels Telegram's
// context; it was lost on the testnet ("sendMessage: context canceled").
func TestShutdownDoesNotCutOffMessageBeingSent(t *testing.T) {
	b := newBot()
	sending := make(chan struct{})
	delivered := make(chan string, 1)
	b.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		if strings.HasSuffix(r.URL.Path, "/setMyCommands") {
			return response(`{"ok":true,"result":true}`), nil
		}
		close(sending)
		time.Sleep(50 * time.Millisecond) // shutdown happens meanwhile
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		require.NoError(t, r.ParseForm())
		delivered <- r.Form.Get("text")
		return response(`{"ok":true,"result":{}}`), nil
	})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { b.Run(ctx); close(done) }()
	b.Notify("Bot stopped")
	<-sending
	cancel()
	<-done
	select {
	case text := <-delivered:
		assert.Equal(t, "Bot stopped", text)
	default:
		t.Fatal("the message in flight was cancelled by shutdown")
	}
}

func TestMessageTruncationPreservesUTF8(t *testing.T) {
	b := newBot()
	b.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		require.NoError(t, r.ParseForm())
		text := r.Form.Get("text")
		assert.True(t, utf8.ValidString(text), "truncation must not split a Unicode character")
		assert.LessOrEqual(t, utf8.RuneCountInString(text), 4096)
		assert.Equal(t, "42", r.Form.Get("chat_id"))
		return response(`{"ok":true,"result":{}}`), nil
	})}
	require.NoError(t, b.send(context.Background(), 42, strings.Repeat("あ", 2000)))
	require.NoError(t, b.send(context.Background(), 42, strings.Repeat("あ", 5000)))
}

func TestPollingIgnoresUnlistedChats(t *testing.T) {
	b := newBot()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	handled, sent, polls := 0, 0, 0
	b.Handle("status", "status", func(context.Context, []string) string { handled++; return "ok" })
	b.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/getUpdates") {
			polls++
			if polls > 1 {
				cancel()
				return nil, context.Canceled
			}
			return response(`{"ok":true,"result":[{"update_id":1,"message":{"chat":{"id":999},"text":"/status"}},{"update_id":2,"message":{"chat":{"id":42},"text":"/status"}}]}`), nil
		}
		sent++
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "42", r.Form.Get("chat_id"))
		return response(`{"ok":true,"result":{}}`), nil
	})}
	b.pollLoop(ctx)
	assert.Equal(t, 1, handled)
	assert.Equal(t, 1, sent)
}

func TestHelpKeepsRegistrationOrderAndIntro(t *testing.T) {
	b := newBot()
	noop := func(context.Context, []string) string { return "" }
	b.Handle("status", "keadaan bot", noop)
	b.Handle("balance", "saldo", noop)
	b.SetHelpIntro("<b>Perintah</b>")
	help := b.dispatch(context.Background(), "/help")
	assert.True(t, strings.HasPrefix(help, "<b>Perintah</b>\n\n"))
	assert.Less(t, strings.Index(help, "/status"), strings.Index(help, "/balance"), "registration order, not alphabetical")
}

func TestCommandMenuIsPublishedToWhitelistedChats(t *testing.T) {
	b := New("fake-token", []int64{42, 7}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.Handle("status", "ringkasan akun", func(context.Context, []string) string { return "" })
	b.Handle("Bad-Name", "not a valid Telegram command", func(context.Context, []string) string { return "" })
	var forms []url.Values
	b.http = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		require.True(t, strings.HasSuffix(r.URL.Path, "/setMyCommands"))
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		forms = append(forms, form)
		return response(`{"ok":true,"result":true}`), nil
	})}
	b.publishCommands(context.Background())
	require.Len(t, forms, 2)
	assert.JSONEq(t, `[{"command":"status","description":"ringkasan akun"},{"command":"help","description":"daftar perintah dan cara pakai"}]`, forms[0].Get("commands"))
	assert.JSONEq(t, `{"type":"chat","chat_id":42}`, forms[0].Get("scope"))
	assert.JSONEq(t, `{"type":"chat","chat_id":7}`, forms[1].Get("scope"))
}
