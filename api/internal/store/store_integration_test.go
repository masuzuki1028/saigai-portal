//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/auth"
	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/store"
	"github.com/masuzuki1028/saigai-portal/api/internal/store/storetest"
)

func TestResolveUserIsStable(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	p := auth.Principal{Issuer: "local", Subject: "alice"}
	u1, err := s.ResolveUser(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	u2, err := s.ResolveUser(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if u1.ID == "" || u1.ID != u2.ID {
		t.Errorf("同じ sub で userId が変わった: %q, %q", u1.ID, u2.ID)
	}
	other, _ := s.ResolveUser(ctx, auth.Principal{Issuer: "local", Subject: "bob"})
	if other.ID == u1.ID {
		t.Error("別の利用者に同じ userId が発行された")
	}
}

func newLocation(owner, id string) store.Location {
	now := time.Now().UTC()
	return store.Location{
		OwnerKey: owner, ID: id, Name: "自宅", RadiusM: 3000, Geohash6: "xn76ur", CoordEnc: []byte{1},
		MuniCode: "15202", AreaName: "長岡市", WarnKey: "15202", QuakeKey: "15202",
		HazardSettings: domain.DefaultHazardSettings(), Version: 1, CreatedAt: now, UpdatedAt: now,
	}
}

func TestLocationLifecycle(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	l := newLocation("OWNER#U#A", "01JAAAAAAAAAAAAAAAAAAAAAAA")
	if err := s.CreateLocation(ctx, l); err != nil {
		t.Fatal(err)
	}
	if es, _ := s.QueryArea(ctx, store.AreaWarn, "15202"); len(es) != 1 || es[0].OwnerKey != l.OwnerKey {
		t.Fatalf("逆引きが作られていない: %+v", es)
	}
	// 他人の所有者キーでは取得できない
	if _, err := s.GetLocation(ctx, "OWNER#U#B", l.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("他人の地点を取得できた: %v", err)
	}

	moved := l
	moved.WarnKey, moved.QuakeKey, moved.Version = "13101", "13101", 2
	if err := s.UpdateLocation(ctx, l, moved); err != nil {
		t.Fatal(err)
	}
	if es, _ := s.QueryArea(ctx, store.AreaWarn, "15202"); len(es) != 0 {
		t.Errorf("古いエリアの逆引きが残っている: %+v", es)
	}
	if es, _ := s.QueryArea(ctx, store.AreaQuake, "13101"); len(es) != 1 {
		t.Errorf("新しいエリアの逆引きがない: %+v", es)
	}
	// 古い version での更新は競合になる
	if err := s.UpdateLocation(ctx, l, moved); !errors.Is(err, store.ErrConflict) {
		t.Errorf("古い version で更新できた: %v", err)
	}
	if err := s.DeleteLocation(ctx, moved); err != nil {
		t.Fatal(err)
	}
	if es, _ := s.QueryArea(ctx, store.AreaQuake, "13101"); len(es) != 0 {
		t.Errorf("削除後も逆引きが残っている: %+v", es)
	}
}

func TestEventIdempotent(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	ev := &domain.Event{ID: "abc", Source: "jma_xml", Kind: domain.EventWarning, IssuedAt: time.Now()}
	first, err := s.PutEventIfAbsent(ctx, ev)
	if err != nil || !first {
		t.Fatalf("1回目: %v, %v", first, err)
	}
	second, err := s.PutEventIfAbsent(ctx, ev)
	if err != nil || second {
		t.Errorf("同じ電文を二度保存した: %v, %v", second, err)
	}
	if ok, _ := s.EventExists(ctx, "abc"); !ok {
		t.Error("EventExists が false")
	}
}

func TestStateIgnoresOlderTelegram(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	t0 := time.Date(2026, 10, 20, 10, 0, 0, 0, time.FixedZone("JST", 9*3600))
	ok, err := s.PutState(ctx, "L1", store.HazardState{Key: "rain", Severity: 3, ReportTime: t0})
	if err != nil || !ok {
		t.Fatalf("PutState: %v %v", ok, err)
	}
	ok, _ = s.PutState(ctx, "L1", store.HazardState{Key: "rain", Severity: 2, ReportTime: t0.Add(-time.Minute)})
	if ok {
		t.Error("古い電文で状態が戻った")
	}
	st, _ := s.GetState(ctx, "L1", "rain")
	if st.Severity != 3 {
		t.Errorf("Severity = %d", st.Severity)
	}
}

func TestTimelinePagingAndCursor(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	base := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		it := store.TimelineItem{ID: string(rune('a' + i)), Kind: "official", Title: "t", IssuedAt: base.Add(time.Duration(i) * time.Minute)}
		if err := s.PutTimelineItem(ctx, "L1", it); err != nil {
			t.Fatal(err)
		}
	}
	_ = s.PutTimelineItem(ctx, "L2", store.TimelineItem{ID: "z", IssuedAt: base})

	page1, next, err := s.QueryTimeline(ctx, "L1", 3, "")
	if err != nil || len(page1) != 3 || next == "" || page1[0].ID != "e" {
		t.Fatalf("1ページ目: %+v next=%q err=%v", page1, next, err)
	}
	page2, _, err := s.QueryTimeline(ctx, "L1", 3, next)
	if err != nil || len(page2) != 2 || page2[1].ID != "a" {
		t.Fatalf("2ページ目: %+v err=%v", page2, err)
	}
	// L1 のカーソルで L2 を読ませない
	if _, _, err := s.QueryTimeline(ctx, "L2", 3, next); !errors.Is(err, store.ErrBadCursor) {
		t.Errorf("別の地点のカーソルを受け付けた: %v", err)
	}
	if _, _, err := s.QueryTimeline(ctx, "L1", 3, "!!"); !errors.Is(err, store.ErrBadCursor) {
		t.Errorf("壊れたカーソルを受け付けた: %v", err)
	}
}
