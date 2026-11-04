package ingest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

type fakeStore struct {
	mu      sync.Mutex
	events  map[string]bool
	cursors map[string]store.IngestCursor
}

func (f *fakeStore) EventExists(_ context.Context, id string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[id], nil
}
func (f *fakeStore) PutEventIfAbsent(_ context.Context, ev *domain.Event) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.events[ev.ID] {
		return false, nil
	}
	f.events[ev.ID] = true
	return true, nil
}
func (f *fakeStore) DeleteEvent(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.events, id)
	return nil
}
func (f *fakeStore) GetCursor(_ context.Context, s string) (store.IngestCursor, error) {
	c, ok := f.cursors[s]
	if !ok {
		c.Source = s
	}
	return c, nil
}
func (f *fakeStore) PutCursor(_ context.Context, c store.IngestCursor) error {
	f.cursors[c.Source] = c
	return nil
}

// 気象庁のサーバーの代わりに fixture を返す（CI から外部サービスを呼ばない）。
func TestPollFetchesEachTelegramOnce(t *testing.T) {
	var mu sync.Mutex
	telegramHits := map[string]int{}
	feed, _ := os.ReadFile("../jma/testdata/feed_extra.xml")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed/extra.xml" {
			if r.Header.Get("If-Modified-Since") == "Tue, 20 Oct 2026 08:00:00 GMT" {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("Last-Modified", "Tue, 20 Oct 2026 08:00:00 GMT")
			_, _ = w.Write([]byte(strings.ReplaceAll(string(feed), "https://www.data.jma.go.jp/developer/xml", srv.URL)))
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/data/")
		mu.Lock()
		telegramHits[name]++
		mu.Unlock()
		b, err := os.ReadFile("../jma/testdata/" + name)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	fs := &fakeStore{events: map[string]bool{}, cursors: map[string]store.IngestCursor{}}
	var got []*domain.Event
	in := New(fs, func(_ context.Context, ev *domain.Event) error { got = append(got, ev); return nil }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	in.Client = srv.Client()
	in.Feeds = []string{srv.URL + "/feed/extra.xml"}

	ctx := context.Background()
	st, err := in.Poll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// feed_extra.xml の 2 件のうち、fixture があるのは富山県の電文だけ（新潟県の電文は 404 → 失敗として数える）
	if st.Entries != 2 || st.New != 1 || st.Failed != 1 || len(got) != 1 {
		t.Errorf("1回目: %+v, events=%d", st, len(got))
	}

	// 2回目はフィードが更新されていない（304）
	st, _ = in.Poll(ctx)
	if st.Entries != 0 {
		t.Errorf("2回目: 304 なのに処理した: %+v", st)
	}

	// カーソルを消してフィードを取り直しても、取り込み済みの電文は再取得しない
	fs.cursors = map[string]store.IngestCursor{}
	st, _ = in.Poll(ctx)
	if st.Skipped != 1 || telegramHits["20261020075747_0_VPWW54_160000.xml"] != 1 {
		t.Errorf("3回目: %+v hits=%v", st, telegramHits)
	}
}

// 後続の処理（match-queue への送信）に失敗した電文は、次の取り込みでもう一度処理する。
func TestIngestRetriesWhenOnEventFails(t *testing.T) {
	data, err := os.ReadFile("../jma/testdata/20261020075747_0_VPWW54_160000.xml")
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeStore{events: map[string]bool{}, cursors: map[string]store.IngestCursor{}}
	calls := 0
	in := New(fs, func(context.Context, *domain.Event) error {
		calls++
		if calls == 1 {
			return errors.New("queue unavailable")
		}
		return nil
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	ctx := context.Background()
	if err := in.Ingest(ctx, data, "t1", &Stats{}); err == nil {
		t.Fatal("1回目: エラーにならない")
	}
	var st Stats
	if err := in.Ingest(ctx, data, "t1", &st); err != nil || st.New != 1 || calls != 2 {
		t.Errorf("2回目: err=%v stats=%+v calls=%d", err, st, calls)
	}
	st = Stats{}
	if err := in.Ingest(ctx, data, "t1", &st); err != nil || st.Skipped != 1 || calls != 2 {
		t.Errorf("3回目: 成功した電文をもう一度処理した: err=%v stats=%+v calls=%d", err, st, calls)
	}
}
