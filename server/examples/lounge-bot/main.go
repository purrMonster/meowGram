// lounge-bot demonstrates durable cursor and reply recovery using only the standard library.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type message struct {
	ID         string    `json:"id"`
	SenderID   string    `json:"sender_id"`
	Username   string    `json:"username"`
	SenderKind string    `json:"sender_kind"`
	Text       string    `json:"text_content"`
	CreatedAt  time.Time `json:"created_at"`
}
type cursor struct {
	After time.Time
	ID    string
}
type pending struct {
	Cursor  cursor
	Key     string
	Body    json.RawMessage
	Started time.Time
}
type state struct {
	Account string
	Cursor  cursor
	Pending *pending
}
type bot struct {
	client                                              *http.Client
	syncURL, sendURL, user, password, selfID, statePath string
	state                                               state
	now                                                 func() time.Time
}
type retryError struct{ delay time.Duration }

func (e *retryError) Error() string { return "transient API failure" }

func (b *bot) load() error {
	account := b.syncURL + "|" + b.sendURL + "|" + b.user + "|" + b.selfID
	b.state = state{Account: account, Cursor: cursor{After: time.Unix(0, 0).UTC()}}
	data, err := os.ReadFile(b.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(data, &b.state); err != nil {
		return errors.New("invalid cursor file")
	}
	if b.state.Account != account {
		return errors.New("cursor belongs to a different server or principal; use a separate state file")
	}
	if b.state.Cursor.After.IsZero() {
		return errors.New("invalid cursor timestamp")
	}
	return nil
}
func (b *bot) save() error {
	data, err := json.Marshal(b.state)
	if err != nil {
		return err
	}
	dir := filepath.Dir(b.statePath)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".cursor-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, b.statePath)
}
func (b *bot) request(ctx context.Context, method, endpoint, key string, body []byte) ([]byte, error) {
	r, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.SetBasicAuth(b.user, b.password)
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
		r.Header.Set("Content-Type", "application/json")
	}
	response, err := b.client.Do(r)
	if err != nil {
		return nil, &retryError{3 * time.Second}
	}
	defer response.Body.Close()
	if response.StatusCode == 429 || response.StatusCode == 503 {
		delay := 3 * time.Second
		if seconds, err := strconv.ParseInt(response.Header.Get("Retry-After"), 10, 64); err == nil && seconds > 0 && seconds <= int64((1<<63-1)/int64(time.Second)) {
			delay = time.Duration(seconds) * time.Second
		}
		return nil, &retryError{delay}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned HTTP %d; cursor retained", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024+1))
	if err != nil {
		return nil, &retryError{3 * time.Second}
	}
	if len(data) > 16*1024*1024 {
		return nil, errors.New("API response too large")
	}
	return data, nil
}
func (b *bot) finishPending(ctx context.Context) error {
	p := b.state.Pending
	if p == nil {
		return nil
	}
	// Never replay after the server's minimum guaranteed retention window.
	if b.now().Sub(p.Started) >= 168*time.Hour {
		return errors.New("pending send exceeded 168h; reconcile manually before resuming")
	}
	if _, err := b.request(ctx, "POST", b.sendURL, p.Key, p.Body); err != nil {
		return err
	}
	b.state.Cursor = p.Cursor
	b.state.Pending = nil
	return b.save()
}
func (b *bot) poll(ctx context.Context) error {
	// A prior process may have committed a send but lost its response.
	if err := b.finishPending(ctx); err != nil {
		return err
	}
	u, err := url.Parse(b.syncURL)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("after", b.state.Cursor.After.UTC().Format(time.RFC3339Nano))
	q.Set("after_id", b.state.Cursor.ID)
	u.RawQuery = q.Encode()
	data, err := b.request(ctx, "GET", u.String(), "", nil)
	if err != nil {
		return err
	}
	var messages []message
	if err = json.Unmarshal(data, &messages); err != nil {
		return errors.New("invalid sync response")
	}
	if len(messages) > 500 {
		return errors.New("sync exceeded page limit")
	}
	for _, m := range messages {
		if m.ID == "" || m.SenderID == "" || m.Username == "" || m.CreatedAt.IsZero() || (m.SenderKind != "bot" && m.SenderKind != "human") {
			return errors.New("invalid sync message")
		}
		c := b.state.Cursor
		if m.CreatedAt.Before(c.After) || (m.CreatedAt.Equal(c.After) && m.ID <= c.ID) {
			return errors.New("sync returned non-increasing cursor")
		}
		next := cursor{After: m.CreatedAt, ID: m.ID}
		if m.SenderKind == "bot" && m.SenderID == b.selfID {
			b.state.Cursor = next
			if err = b.save(); err != nil {
				return err
			}
			continue
		}
		body, _ := json.Marshal(map[string]string{"text_content": "@" + m.Username + " received"})
		b.state.Pending = &pending{Cursor: next, Key: "pourover-reply-" + m.ID, Body: body, Started: b.now()}
		if err = b.save(); err != nil {
			return err
		}
		if err = b.finishPending(ctx); err != nil {
			return err
		}
	}
	return nil
}
func endpoint(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == ""
}
func run() error {
	b := &bot{syncURL: os.Getenv("MEOWGRAM_SYNC_URL"), sendURL: os.Getenv("MEOWGRAM_SEND_URL"), user: os.Getenv("MEOWGRAM_USERNAME"), selfID: os.Getenv("MEOWGRAM_BOT_ID"), statePath: os.Getenv("MEOWGRAM_STATE_FILE"), now: time.Now}
	if !endpoint(b.syncURL) || !endpoint(b.sendURL) || b.user == "" || b.selfID == "" || b.statePath == "" || os.Getenv("MEOWGRAM_CHANNEL") != "" {
		return errors.New("set HTTPS sync/send URLs, username, bot ID and state file; keep MEOWGRAM_CHANNEL empty")
	}
	password, err := os.ReadFile(os.Getenv("MEOWGRAM_PASSWORD_FILE"))
	if err != nil {
		return errors.New("cannot read MEOWGRAM_PASSWORD_FILE")
	}
	b.password = strings.TrimRight(string(password), "\r\n")
	if b.password == "" {
		return errors.New("empty password file")
	}
	// Never follow redirects with credentials. System trust roots verify TLS.
	b.client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if err = b.load(); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	backoff := 3 * time.Second
	for {
		err = b.poll(ctx)
		if ctx.Err() != nil {
			return nil
		}
		wait := 3 * time.Second
		if err != nil {
			var retry *retryError
			if !errors.As(err, &retry) {
				return err
			}
			wait = max(backoff, retry.delay)
			backoff = min(60*time.Second, backoff*2)
		} else {
			backoff = 3 * time.Second
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
