package matcher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// memStore は matcher の Store をメモリで代替する。failPut を立てると PutTimelineItem が失敗する。
type memStore struct {
	entries []store.AreaEntry
	states  map[[2]string]store.HazardState
	items   map[string]store.TimelineItem
	failPut bool
}

func newMemStore(entries ...store.AreaEntry) *memStore {
	return &memStore{entries: entries, states: map[[2]string]store.HazardState{}, items: map[string]store.TimelineItem{}}
}

func (s *memStore) QueryArea(context.Context, store.AreaKind, string) ([]store.AreaEntry, error) {
	return s.entries, nil
}

func (s *memStore) GetState(_ context.Context, locID, key string) (store.HazardState, error) {
	if st, ok := s.states[[2]string{locID, key}]; ok {
		return st, nil
	}
	return store.HazardState{Key: key}, nil
}

// PutState は本物と同じく、保存済みより古い電文では書き込まない。
func (s *memStore) PutState(_ context.Context, locID string, st store.HazardState) (bool, error) {
	k := [2]string{locID, st.Key}
	if old, ok := s.states[k]; ok && st.ReportTime.Before(old.ReportTime) {
		return false, nil
	}
	s.states[k] = st
	return true, nil
}

func (s *memStore) PutTimelineItem(_ context.Context, locID string, it store.TimelineItem) error {
	if s.failPut {
		return errors.New("timeline: 書き込みに失敗")
	}
	s.items[locID+"/"+it.ID] = it
	return nil
}

func (s *memStore) PutAffected(context.Context, store.Affected) error { return nil }
func (s *memStore) ListAffected(context.Context, string) ([]store.Affected, error) {
	return nil, nil
}
func (s *memStore) CancelTimelineItem(context.Context, string, string, time.Time) error { return nil }
func (s *memStore) PutQuakeKey(context.Context, string, string) error                   { return nil }
func (s *memStore) GetQuakeKey(context.Context, string) (string, error)                 { return "", nil }

func quakeEvent() *domain.Event {
	at := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	return &domain.Event{
		ID: "E1", Source: "jma_xml", Kind: domain.EventEarthquake, InfoType: "発表", IssuedAt: at,
		Quake: &domain.Quake{
			EventID: "Q1", Key: "202611101200:石狩地方北部", OriginTime: at, Hypocenter: "石狩地方北部", Magnitude: "M5.0",
			MaxInt: "5-", Cities: []domain.CityIntensity{{Code: "0120600", Name: "札幌市北区", MaxInt: "5-"}},
		},
	}
}

func homeEntry() store.AreaEntry {
	return store.AreaEntry{LocationID: "L1", OwnerKey: "OWNER#U#U1", Name: "自宅", HazardSettings: domain.DefaultHazardSettings()}
}

// 通知の依頼の送信やその後の書き込みに失敗して同じ電文をやり直しても、通知が落ちないこと。
func TestMatchRetryDoesNotLoseNotification(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sendErr := errors.New("sqs: 送信に失敗")

	t.Run("通知の依頼の送信に失敗", func(t *testing.T) {
		st := newMemStore(homeEntry())
		var sent [][]Job
		fail := true
		m := &Matcher{Store: st, Log: log, Notify: func(_ context.Context, jobs []Job) error {
			if fail {
				return sendErr
			}
			sent = append(sent, jobs)
			return nil
		}}
		if _, err := m.Match(context.Background(), quakeEvent()); !errors.Is(err, sendErr) {
			t.Fatalf("1回目: err = %v, want %v", err, sendErr)
		}
		if len(st.states) != 0 || len(st.items) != 0 {
			t.Fatalf("送信に失敗したのに書き込んだ: states=%d items=%d", len(st.states), len(st.items))
		}
		fail = false
		if _, err := m.Match(context.Background(), quakeEvent()); err != nil {
			t.Fatalf("2回目: %v", err)
		}
		if len(sent) != 1 || len(sent[0]) != 1 || sent[0][0].LocationID != "L1" {
			t.Fatalf("やり直しで通知が送られていない: %+v", sent)
		}
		if len(st.items) != 1 {
			t.Errorf("タイムラインの項目 = %d, want 1", len(st.items))
		}
	})

	t.Run("通知のあとの書き込みに失敗", func(t *testing.T) {
		st := newMemStore(homeEntry())
		st.failPut = true
		var sent int
		m := &Matcher{Store: st, Log: log, Notify: func(context.Context, []Job) error { sent++; return nil }}
		if _, err := m.Match(context.Background(), quakeEvent()); err == nil {
			t.Fatal("1回目: 書き込みの失敗が返らない")
		}
		st.failPut = false
		if _, err := m.Match(context.Background(), quakeEvent()); err != nil {
			t.Fatalf("2回目: %v", err)
		}
		// 1回目に送った通知は、やり直しでもう一度送る（落とさない代わりに重複する）
		if sent != 2 {
			t.Errorf("通知の回数 = %d, want 2", sent)
		}
		if len(st.items) != 1 {
			t.Errorf("タイムラインの項目 = %d, want 1", len(st.items))
		}
	})

	t.Run("処理が終わった電文をもう一度処理しても通知しない", func(t *testing.T) {
		st := newMemStore(homeEntry())
		var sent int
		m := &Matcher{Store: st, Log: log, Notify: func(context.Context, []Job) error { sent++; return nil }}
		for range 2 {
			if _, err := m.Match(context.Background(), quakeEvent()); err != nil {
				t.Fatal(err)
			}
		}
		if sent != 1 {
			t.Errorf("通知の回数 = %d, want 1", sent)
		}
	})
}
