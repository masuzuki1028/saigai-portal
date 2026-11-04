package p2p

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/jma"
)

func TestParse(t *testing.T) {
	data, err := os.ReadFile("testdata/history_551.json")
	if err != nil {
		t.Fatal(err)
	}
	events, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 {
		t.Fatal("地震情報が取り出せない")
	}
	for i := 1; i < len(events); i++ {
		if events[i].IssuedAt.Before(events[i-1].IssuedAt) {
			t.Fatal("古い順に並んでいない")
		}
	}
	var kushiro *domain.Event
	for _, ev := range events {
		if ev.Quake.Hypocenter == "釧路沖" {
			kushiro = ev
		}
	}
	if kushiro == nil {
		t.Fatal("釧路沖の地震がない")
	}
	want := []domain.CityIntensity{
		{Code: "0120600", Name: "釧路市", MaxInt: "2"},
		{Code: "0166100", Name: "釧路町", MaxInt: "1"},
		{Code: "0166300", Name: "浜中町", MaxInt: "1"},
		{Code: "0166400", Name: "標茶町", MaxInt: "1"},
		{Code: "0166800", Name: "白糠町", MaxInt: "1"},
	}
	if diff := cmp.Diff(want, kushiro.Quake.Cities); diff != "" {
		t.Errorf("市区町村ごとの震度 (-want +got):\n%s", diff)
	}
	if kushiro.Quake.Epicenter == nil || kushiro.Source != Source {
		t.Errorf("震央・情報源: %+v %q", kushiro.Quake.Epicenter, kushiro.Source)
	}
}

// 同じ地震を気象庁 XML と P2P地震情報の両方から受け取っても、同じ識別子になること。
func TestQuakeKeyMatchesJMA(t *testing.T) {
	xml, err := os.ReadFile("../jma/testdata/20261019205328_0_VXSE53_010000.xml")
	if err != nil {
		t.Fatal(err)
	}
	j, err := jma.Parse(xml, "x")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile("testdata/history_551.json")
	events, _ := Parse(data)
	for _, ev := range events {
		if ev.Quake.Key == j.Quake.Key {
			if diff := cmp.Diff(j.Quake.Cities, ev.Quake.Cities); diff != "" {
				t.Errorf("市区町村の震度が気象庁 XML と違う (-jma +p2p):\n%s", diff)
			}
			return
		}
	}
	t.Errorf("気象庁 XML と同じ識別子（%q）の地震がない", j.Quake.Key)
}

type fakeStore map[string]bool

func (f fakeStore) PutEventIfAbsent(_ context.Context, ev *domain.Event) (bool, error) {
	if f[ev.ID] {
		return false, nil
	}
	f[ev.ID] = true
	return true, nil
}

func (f fakeStore) DeleteEvent(_ context.Context, id string) error {
	delete(f, id)
	return nil
}

func TestIngestIsIdempotent(t *testing.T) {
	data, _ := os.ReadFile("testdata/history_551.json")
	got := 0
	in := &Ingester{Store: fakeStore{}, OnEvent: func(context.Context, *domain.Event) error { got++; return nil }}
	n1, err := in.Ingest(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	n2, _ := in.Ingest(context.Background(), data)
	if n1 == 0 || n2 != 0 || got != n1 {
		t.Errorf("1回目 %d 件、2回目 %d 件、OnEvent %d 回", n1, n2, got)
	}
}

func FuzzParse(f *testing.F) {
	data, _ := os.ReadFile("testdata/history_551.json")
	f.Add(data)
	f.Add([]byte(`[{"code":551,"earthquake":{"time":"x"}}]`))
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = Parse(b) })
}
