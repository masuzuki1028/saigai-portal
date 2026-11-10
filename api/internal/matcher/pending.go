package matcher

import (
	"context"
	"time"

	"github.com/masuzuki1028/saigai-portal/api/internal/store"
)

// pending は1つの電文を処理するあいだの書き込みを貯める Store（Matcher.Match）。
// 状態の読み取りは、貯めた状態を先に見る（同じ電文の中の判定は、書き込んだ場合と同じになる）。
// 地震の識別子の対応（PutQuakeKey）は、通知と関係がないためそのまま書き込む。
type pending struct {
	Store
	states map[[2]string]store.HazardState // [地点, 状態のキー]
	order  [][2]string                     // 状態を書く順
	writes []func(ctx context.Context, s Store) error
}

func newPending(s Store) *pending {
	return &pending{Store: s, states: map[[2]string]store.HazardState{}}
}

func (p *pending) GetState(ctx context.Context, locID, key string) (store.HazardState, error) {
	if st, ok := p.states[[2]string{locID, key}]; ok {
		return st, nil
	}
	return p.Store.GetState(ctx, locID, key)
}

// PutState は貯めるだけで true を返す。古い電文かどうかは、読み取った状態で判定済み。
func (p *pending) PutState(_ context.Context, locID string, st store.HazardState) (bool, error) {
	k := [2]string{locID, st.Key}
	if _, ok := p.states[k]; !ok {
		p.order = append(p.order, k)
	}
	p.states[k] = st
	return true, nil
}

func (p *pending) PutTimelineItem(_ context.Context, locID string, it store.TimelineItem) error {
	p.writes = append(p.writes, func(ctx context.Context, s Store) error { return s.PutTimelineItem(ctx, locID, it) })
	return nil
}

func (p *pending) PutAffected(_ context.Context, a store.Affected) error {
	p.writes = append(p.writes, func(ctx context.Context, s Store) error { return s.PutAffected(ctx, a) })
	return nil
}

func (p *pending) CancelTimelineItem(_ context.Context, locID, itemID string, issuedAt time.Time) error {
	p.writes = append(p.writes, func(ctx context.Context, s Store) error { return s.CancelTimelineItem(ctx, locID, itemID, issuedAt) })
	return nil
}

// flush は貯めた書き込みを行う。タイムラインの項目・取消の記録を先に書き、状態は最後に書く。
// 状態は「この電文の処理が終わった」印になるため、途中で失敗しても、やり直しで判定し直して書き直せる
// （項目は電文と地点から決まる ID で上書きするため重複しない）。
// 同時に処理された新しい電文が先に状態を書いていると、状態は書かれないが、古い電文の項目は残る
// （実際に発表された情報で、タイムラインは発表時刻の順に並ぶ）。
func (p *pending) flush(ctx context.Context) error {
	for _, w := range p.writes {
		if err := w(ctx, p.Store); err != nil {
			return err
		}
	}
	for _, k := range p.order {
		if _, err := p.Store.PutState(ctx, k[0], p.states[k]); err != nil {
			return err
		}
	}
	return nil
}
