// Package ingest は気象庁の Atom フィードを取得し、新しい電文を取り込む（docs/05_system_architecture.md 3.3）。
package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/jma"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// Store は ingest が使う store の操作。
type Store interface {
	EventExists(ctx context.Context, id string) (bool, error)
	PutEventIfAbsent(ctx context.Context, ev *domain.Event) (bool, error)
	GetCursor(ctx context.Context, source string) (store.IngestCursor, error)
	PutCursor(ctx context.Context, c store.IngestCursor) error
}

// Ingester はフィードを取得して電文を取り込む。OnEvent には新しく保存した Event を渡す
// （本番は match-queue への送信、ローカルは matcher の直接呼び出し）。
type Ingester struct {
	Store   Store
	Client  *http.Client
	Feeds   []string // jma.FeedExtra, jma.FeedEqVol。テストでは httptest.Server の URL
	OnEvent func(ctx context.Context, ev *domain.Event) error
	Log     *slog.Logger
	// MaxTelegramBytes は電文1件の上限（大きすぎる応答で止まらないようにする）
	MaxTelegramBytes int64
}

func New(s Store, onEvent func(context.Context, *domain.Event) error, log *slog.Logger) *Ingester {
	return &Ingester{
		Store: s, Client: &http.Client{Timeout: 10 * time.Second}, Feeds: []string{jma.FeedExtra, jma.FeedEqVol},
		OnEvent: onEvent, Log: log, MaxTelegramBytes: 2 << 20,
	}
}

// Stats は1回の取得の結果。
type Stats struct {
	Feeds, Entries, New, Skipped, Failed int
}

// Poll は各フィードを1回ずつ取得する。気象庁の利用上の注意に従い、一度取り込んだ電文は再取得しない
// （フィードは If-Modified-Since で差分だけ取得し、電文は EventExists で確認してから取得する）。
func (in *Ingester) Poll(ctx context.Context) (Stats, error) {
	var st Stats
	var errs []error
	for _, feed := range in.Feeds {
		if err := in.pollFeed(ctx, feed, &st); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", feed, err))
		}
	}
	return st, errors.Join(errs...)
}

func (in *Ingester) pollFeed(ctx context.Context, feed string, st *Stats) error {
	src := "jma:" + feed
	cur, err := in.Store.GetCursor(ctx, src)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return err
	}
	if cur.LastModified != "" {
		req.Header.Set("If-Modified-Since", cur.LastModified)
	}
	res, err := in.Client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	st.Feeds++
	if res.StatusCode == http.StatusNotModified {
		return nil
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("feed status %d", res.StatusCode)
	}
	entries, err := jma.ParseFeed(io.LimitReader(res.Body, 20<<20))
	if err != nil {
		return err
	}
	// 古い順に処理する（同じ地点の状態が新しい電文で上書きされるように）
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if !jma.Wanted(e.Title) {
			continue
		}
		st.Entries++
		if err := in.ingestEntry(ctx, e, st); err != nil {
			st.Failed++
			in.Log.Error("ingest failed", "telegram", e.ID, "err", err)
		}
	}
	cur.LastModified = res.Header.Get("Last-Modified")
	cur.LastFetched = time.Now().UTC()
	return in.Store.PutCursor(ctx, cur)
}

func (in *Ingester) ingestEntry(ctx context.Context, e jma.FeedEntry, st *Stats) error {
	id := jma.EventID(e.ID)
	exists, err := in.Store.EventExists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		st.Skipped++
		return nil
	}
	data, err := in.fetch(ctx, e.URL)
	if err != nil {
		return err
	}
	return in.Ingest(ctx, data, e.ID, st)
}

// Ingest は電文1件を解析・保存し、新しい電文なら OnEvent を呼ぶ。replay からも使う。
func (in *Ingester) Ingest(ctx context.Context, data []byte, telegramID string, st *Stats) error {
	ev, err := jma.Parse(data, telegramID)
	if errors.Is(err, jma.ErrUnsupported) {
		st.Skipped++
		return nil
	}
	if err != nil {
		return err
	}
	created, err := in.Store.PutEventIfAbsent(ctx, ev)
	if err != nil {
		return err
	}
	if !created {
		st.Skipped++
		return nil
	}
	st.New++
	if in.OnEvent != nil {
		return in.OnEvent(ctx, ev)
	}
	return nil
}

func (in *Ingester) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := in.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram status %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, in.MaxTelegramBytes))
}
