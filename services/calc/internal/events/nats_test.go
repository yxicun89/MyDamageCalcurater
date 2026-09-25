//go:build nats

package events

// 実 NATS(JetStream)を使うテスト(ADR-0212 AC-N3・AC-N4)。`make test-nats` だけが実行する
// (`make test` には含めない)。CALC_TEST_NATS_URL が無い・NATS に届かないときは失敗する
// (黙ってスキップして成功扱いにしない。pokedex-svc の POKEDEX_TEST_DSN と同じ流儀)。
// ローカルは `scripts/nats-local-up.sh` で起動する。

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/calcevents"
)

func testNATSURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("CALC_TEST_NATS_URL")
	if url == "" {
		t.Fatal("CALC_TEST_NATS_URL が無い(make test-nats は NATS を前提にする。スキップしない)")
	}
	return url
}

// waitReady は Publisher のストリーム作成(背景の再試行)が終わるまで待つ。
func waitReady(t *testing.T, p *Publisher) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if p.ready.Load() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Publisher のストリーム作成が10秒以内に終わらない")
}

// deleteStreamIfExists はテストの前後でストリームを消し、前回の残りに左右されないようにする。
func deleteStreamIfExists(t *testing.T, url string) {
	t.Helper()
	// New(url) は使わない(背景の ensureStreamLoop が同時に CreateOrUpdateStream を試み、
	// この関数が先に nc.Close() すると "connection closed" の警告ログが紛れ込むため、
	// ここでは削除専用の素の接続を作る)。
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("NATS に接続できない: %v", err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("JetStream を初期化できない: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = js.DeleteStream(ctx, calcevents.StreamName) // 無ければ何もしない
}

// AC-N3・AC-N4: 実 NATS に対して Publish すると、consumer が無くてもストリームにメッセージが
// 保存される(Limits retention。SkipMsgNoInterest は Interest retention のときだけ効く)。
func TestPublishStoresMessageWithoutConsumer(t *testing.T) {
	url := testNATSURL(t)
	deleteStreamIfExists(t, url)

	p := New(url)
	if p == nil {
		t.Fatal("New が nil を返した(接続できない)")
	}
	defer p.Shutdown()
	waitReady(t, p)

	occurredAt := time.Now().UTC().Truncate(time.Millisecond)
	detail := &calcevents.CalcDetail{
		Format:     "single",
		Attacker:   api.Individual{SpeciesKey: "0001-000", NatureId: "adamant", Sp: api.StatBlock{}},
		Defender:   api.Individual{SpeciesKey: "0002-000", NatureId: "adamant", Sp: api.StatBlock{}},
		MoveID:     "tackle",
		MinPercent: 12.3,
		MaxPercent: 14.5,
	}
	p.Publish("device-test-1", "session-test-1", calcevents.OperationCalc, occurredAt, detail)

	// 発行の goroutine(非同期)が確定するのを待つ。Shutdown が確実に待つので、ここでは
	// Shutdown を先に呼んでから内容を確認する(defer より先に呼び、確認後にもう一度 defer が
	// 呼ばれても nil セーフではないため、確認は Shutdown 前に MsgGet で行う)。
	deadline := time.Now().Add(5 * time.Second)
	var raw jetstream.RawStreamMsg
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		stream, err := p.js.Stream(ctx, calcevents.StreamName)
		if err == nil {
			msg, err2 := stream.GetLastMsgForSubject(ctx, calcevents.Subject("device-test-1"))
			cancel()
			if err2 == nil {
				raw = *msg
				lastErr = nil
				break
			}
			lastErr = err2
		} else {
			cancel()
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("発行したメッセージを取得できない: %v", lastErr)
	}

	var got calcevents.Event
	if err := json.Unmarshal(raw.Data, &got); err != nil {
		t.Fatalf("メッセージの JSON を復元できない: %v", err)
	}
	if got.DeviceID != "device-test-1" || got.SessionID != "session-test-1" {
		t.Errorf("envelope が一致しない: %+v", got)
	}
	if got.Operation != calcevents.OperationCalc {
		t.Errorf("Operation = %q, want %q", got.Operation, calcevents.OperationCalc)
	}
	if !got.OccurredAt.Equal(occurredAt) {
		t.Errorf("OccurredAt = %v, want %v", got.OccurredAt, occurredAt)
	}
	if got.Detail == nil || got.Detail.MoveID != "tackle" {
		t.Errorf("Detail が一致しない: %+v", got.Detail)
	}

	info, err := func() (*jetstream.StreamInfo, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		stream, err := p.js.Stream(ctx, calcevents.StreamName)
		if err != nil {
			return nil, err
		}
		return stream.Info(ctx)
	}()
	if err != nil {
		t.Fatalf("StreamInfo を取得できない: %v", err)
	}
	if info.Config.Retention != jetstream.LimitsPolicy {
		t.Errorf("Retention = %v, want LimitsPolicy", info.Config.Retention)
	}
	if info.Config.MaxAge != 7*24*time.Hour {
		t.Errorf("MaxAge = %v, want 7日", info.Config.MaxAge)
	}
	if info.State.Msgs == 0 {
		t.Error("consumer が無いのにメッセージが保存されていない(Interest retention の混入を疑う)")
	}
}

// AC-N3: Detail の無い envelope だけのイベント(calcBulk/calcReverse)も発行できる。
func TestPublishEnvelopeOnly(t *testing.T) {
	url := testNATSURL(t)
	deleteStreamIfExists(t, url)

	p := New(url)
	if p == nil {
		t.Fatal("New が nil を返した(接続できない)")
	}
	defer p.Shutdown()
	waitReady(t, p)

	occurredAt := time.Now().UTC().Truncate(time.Millisecond)
	p.Publish("device-test-2", "session-test-2", calcevents.OperationBulk, occurredAt, nil)

	deadline := time.Now().Add(5 * time.Second)
	var raw *jetstream.RawStreamMsg
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		stream, err := p.js.Stream(ctx, calcevents.StreamName)
		if err == nil {
			if msg, err2 := stream.GetLastMsgForSubject(ctx, calcevents.Subject("device-test-2")); err2 == nil {
				raw = msg
				cancel()
				break
			}
		}
		cancel()
		time.Sleep(100 * time.Millisecond)
	}
	if raw == nil {
		t.Fatal("発行したメッセージを取得できない")
	}
	var got calcevents.Event
	if err := json.Unmarshal(raw.Data, &got); err != nil {
		t.Fatalf("メッセージの JSON を復元できない: %v", err)
	}
	if got.Detail != nil {
		t.Errorf("Detail が入っている(envelope だけのはず): %+v", got.Detail)
	}
	if got.Operation != calcevents.OperationBulk {
		t.Errorf("Operation = %q, want %q", got.Operation, calcevents.OperationBulk)
	}
}
