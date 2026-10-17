// Package telegram sends notifications to Telegram chats and serves bot
// commands through the Bot API (long polling). Only whitelisted chats can
// run commands.
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	apiBase      = "https://api.telegram.org/bot"
	pollTimeout  = 50 // seconds
	sendInterval = 1100 * time.Millisecond
	queueSize    = 200
	maxMessage   = 4000 // Telegram limit is 4096 characters
)

// CommandFunc handles a command and returns the reply (HTML allowed).
type CommandFunc func(ctx context.Context, args []string) string

type command struct {
	help string
	fn   CommandFunc
}

// Bot is a Telegram bot client.
type Bot struct {
	token   string
	chatIDs []int64
	http    *http.Client
	log     *slog.Logger

	mu       sync.Mutex
	commands map[string]command
	queue    chan string
}

func New(token string, chatIDs []int64, log *slog.Logger) *Bot {
	return &Bot{
		token: token, chatIDs: chatIDs, log: log.With("component", "telegram"),
		http:     &http.Client{Timeout: (pollTimeout + 15) * time.Second},
		commands: map[string]command{},
		queue:    make(chan string, queueSize),
	}
}

// Handle registers a command, e.g. Handle("status", "show status", fn).
func (b *Bot) Handle(name, help string, fn CommandFunc) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commands[name] = command{help: help, fn: fn}
}

// Notify queues a message (HTML) for every whitelisted chat. It never
// blocks; messages are dropped when the queue is full.
func (b *Bot) Notify(text string) {
	select {
	case b.queue <- text:
	default:
		b.log.Warn("telegram queue full, dropping message")
	}
}

// Run sends queued messages and serves commands until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Go(func() { b.sendLoop(ctx) })
	wg.Go(func() { b.pollLoop(ctx) })
	wg.Wait()
}

func (b *Bot) sendLoop(ctx context.Context) {
	tick := time.NewTicker(sendInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			// Best-effort flush (e.g. the shutdown message).
			flush, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for {
				select {
				case text := <-b.queue:
					b.sendAll(flush, text)
				default:
					return
				}
			}
		case text := <-b.queue:
			b.sendAll(ctx, text)
			<-tick.C
		}
	}
}

func (b *Bot) sendAll(ctx context.Context, text string) {
	for _, id := range b.chatIDs {
		if err := b.send(ctx, id, text); err != nil {
			b.log.Warn("telegram send failed", "chat", id, "err", err)
		}
	}
}

func (b *Bot) send(ctx context.Context, chatID int64, text string) error {
	if len(text) > maxMessage {
		text = text[:maxMessage] + "…"
	}
	form := url.Values{
		"chat_id":                  {strconv.FormatInt(chatID, 10)},
		"text":                     {text},
		"parse_mode":               {"HTML"},
		"disable_web_page_preview": {"true"},
	}
	return b.call(ctx, "sendMessage", form, nil)
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

func (b *Bot) pollLoop(ctx context.Context) {
	var offset int64
	backoff := time.Second
	for ctx.Err() == nil {
		var ups []update
		form := url.Values{"timeout": {strconv.Itoa(pollTimeout)}, "allowed_updates": {`["message"]`}}
		if offset > 0 {
			form.Set("offset", strconv.FormatInt(offset, 10))
		}
		if err := b.call(ctx, "getUpdates", form, &ups); err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Warn("telegram poll failed", "err", err, "retry_in", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, time.Minute)
			continue
		}
		backoff = time.Second
		for _, u := range ups {
			offset = u.UpdateID + 1
			if u.Message == nil || !strings.HasPrefix(u.Message.Text, "/") {
				continue
			}
			chat := u.Message.Chat.ID
			if !slices.Contains(b.chatIDs, chat) {
				b.log.Warn("telegram command from unknown chat ignored", "chat", chat)
				continue
			}
			reply := b.dispatch(ctx, u.Message.Text)
			if err := b.send(ctx, chat, reply); err != nil {
				b.log.Warn("telegram reply failed", "err", err)
			}
		}
	}
}

func (b *Bot) dispatch(ctx context.Context, text string) string {
	fields := strings.Fields(text)
	name := strings.TrimPrefix(fields[0], "/")
	name, _, _ = strings.Cut(name, "@") // "/status@MyBot" in groups
	b.mu.Lock()
	cmd, ok := b.commands[name]
	b.mu.Unlock()
	if name == "help" || name == "start" || !ok {
		return b.help()
	}
	b.log.Info("telegram command", "command", name, "args", fields[1:])
	return cmd.fn(ctx, fields[1:])
}

func (b *Bot) help() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	names := make([]string, 0, len(b.commands))
	for n := range b.commands {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	sb.WriteString("<b>Commands</b>\n")
	for _, n := range names {
		fmt.Fprintf(&sb, "/%s — %s\n", n, html.EscapeString(b.commands[n].help))
	}
	return sb.String()
}

func (b *Bot) call(ctx context.Context, method string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+b.token+"/"+method, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := b.http.Do(req)
	if err != nil {
		// The URL contains the token; never log it.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			return fmt.Errorf("%s: %w", method, uerr.Err)
		}
		return fmt.Errorf("%s: request failed", method)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var r struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return fmt.Errorf("%s: decode: %w", method, err)
	}
	if !r.OK {
		return fmt.Errorf("%s: %s", method, r.Description)
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

// Escape escapes text for HTML messages.
func Escape(s string) string { return html.EscapeString(s) }
