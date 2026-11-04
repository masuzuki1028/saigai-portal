package queue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/go-cmp/cmp"

	"github.com/masuzuki1028/saigai-portal/api/internal/domain"
	"github.com/masuzuki1028/saigai-portal/api/internal/jma"
	"github.com/masuzuki1028/saigai-portal/api/internal/p2p"
)

type fakeSQS struct{ bodies []string }

func (f *fakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.bodies = append(f.bodies, *in.MessageBody)
	return &sqs.SendMessageOutput{}, nil
}

// 取り込んだ電文は、match-queue を通っても同じ内容で matcher に届く。
func TestEventRoundTrip(t *testing.T) {
	files, _ := filepath.Glob("../jma/testdata/*_V*.xml")
	var evs []*domain.Event
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		ev, err := jma.Parse(data, filepath.Base(f))
		if errors.Is(err, jma.ErrUnsupported) {
			continue // 訓練の電文など
		}
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		evs = append(evs, ev)
	}
	data, _ := os.ReadFile("../p2p/testdata/history_551.json")
	pe, err := p2p.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	evs = append(evs, pe...)
	if len(evs) < 5 {
		t.Fatalf("fixture が少ない: %d", len(evs))
	}

	f := &fakeSQS{}
	s := Sender{Client: f, URL: "https://sqs.example/match"}
	for _, ev := range evs {
		if err := s.Send(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
	}
	var sqsEv events.SQSEvent
	for i, b := range f.bodies {
		sqsEv.Records = append(sqsEv.Records, events.SQSMessage{MessageId: string(rune('a' + i)), Body: b})
	}
	var got []domain.Event
	res := HandleBatch(context.Background(), sqsEv, slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ context.Context, ev domain.Event) error {
		got = append(got, ev)
		return nil
	})
	if len(res.BatchItemFailures) != 0 || len(got) != len(evs) {
		t.Fatalf("failures=%v got=%d", res.BatchItemFailures, len(got))
	}
	for i := range evs {
		if d := cmp.Diff(*evs[i], got[i]); d != "" {
			t.Errorf("%s (-sent +received):\n%s", evs[i].Title, d)
		}
	}
}

func TestHandleBatchReportsFailures(t *testing.T) {
	ev := events.SQSEvent{Records: []events.SQSMessage{
		{MessageId: "ok", Body: `{"n":1}`},
		{MessageId: "fail", Body: `{"n":2}`},
		{MessageId: "broken", Body: `{`},
	}}
	type msg struct{ N int }
	res := HandleBatch(context.Background(), ev, slog.New(slog.NewTextHandler(io.Discard, nil)), func(_ context.Context, m msg) error {
		if m.N == 2 {
			return errors.New("boom")
		}
		return nil
	})
	var ids []string
	for _, f := range res.BatchItemFailures {
		ids = append(ids, f.ItemIdentifier)
	}
	if strings.Join(ids, ",") != "fail,broken" {
		t.Errorf("failures = %v", ids)
	}
}

func TestSendRejectsLargeMessage(t *testing.T) {
	f := &fakeSQS{}
	err := Sender{Client: f, URL: "u"}.Send(context.Background(), strings.Repeat("a", MaxMessageBytes))
	if err == nil || len(f.bodies) != 0 {
		t.Errorf("大きすぎるメッセージを送った: %v", err)
	}
}
