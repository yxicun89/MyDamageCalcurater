// Package events は calc-svc から NATS JetStream への計算イベント発行を持つ(ADR-0212)。
//
// 発行はリクエストの応答を絶対にブロックしない(CLAUDE.md 絶対ルール5)。NATS に接続できない・
// ストリームが無い・発行が失敗しても、呼び出し元(httpapi ハンドラ)には一切影響しない
// (Publish はエラーを返さない。失敗はログの warn にだけ残す)。
package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"example.com/pokecalc/services/internal/calcevents"
)

const (
	// publishTimeout は1件の発行の確認(PubAckFuture)を待つ上限(ADR-0212 §6)。
	publishTimeout = 2 * time.Second
	// shutdownDrainTimeout は終了時、発行 goroutine と JetStream の確認待ちそれぞれに与える上限
	// (ADR-0212 §6。合計の終了予算は http.Server の shutdownTimeout に、この値の2倍を足したもの)。
	shutdownDrainTimeout = 2 * time.Second

	// streamMaxBytes はストリームの容量上限(ADR-0212 §4)。512MiB。
	streamMaxBytes = 512 * 1024 * 1024
	// streamMaxAge はストリームのメッセージ保持期間(ADR-0209 §3 #6・§7。7日)。
	streamMaxAge = 7 * 24 * time.Hour

	// createStreamRetryInitial・createStreamRetryMax はストリーム作成(CreateOrUpdateStream)の
	// 再試行間隔(ADR-0212 §5。services/calc/cmd/calc/main.go の fetchMasterLoop と同じ型。
	// ADR-0204 §3)。
	createStreamRetryInitial = 500 * time.Millisecond
	createStreamRetryMax     = 30 * time.Second
)

// streamConfig はストリームの設定そのもの(ADR-0212 §4)。
var streamConfig = jetstream.StreamConfig{
	Name:      calcevents.StreamName,
	Subjects:  []string{calcevents.SubjectPrefix + "*"},
	Storage:   jetstream.FileStorage,
	Replicas:  1,
	MaxAge:    streamMaxAge,
	MaxBytes:  streamMaxBytes,
	Discard:   jetstream.DiscardOld,
	Retention: jetstream.LimitsPolicy,
}

// Publisher は calc-svc から NATS JetStream への発行を1つにまとめる。ゼロ値は使わない
// (New で作る)。nil の *Publisher に対する呼び出しも安全(発行しないだけ)。
type Publisher struct {
	nc *nats.Conn
	js jetstream.JetStream

	ready atomic.Bool // ストリーム作成が一度でも成功したら true(以後は戻さない)

	wg sync.WaitGroup // 発行中の goroutine の数
}

// New は CALC_NATS_URL(空なら発行を丸ごと無効化)から Publisher を作る。NATS に接続できない・
// ストリームを作成できなくても、この関数自体は失敗しない(呼び出し元の起動をブロックしない。
// CLAUDE.md 絶対ルール5・ADR-0212 §5)。
func New(natsURL string) *Publisher {
	if natsURL == "" {
		return nil
	}
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		// RetryOnFailedConnect(true) を付けても、URL 自体が不正な場合はここで失敗する。
		slog.Warn("calc-svc: NATS に接続できない。イベント発行を無効化する", "error", err)
		return nil
	}
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Warn("calc-svc: JetStream を初期化できない。イベント発行を無効化する", "error", err)
		nc.Close()
		return nil
	}

	p := &Publisher{nc: nc, js: js}
	go p.ensureStreamLoop(context.Background())
	return p
}

// ensureStreamLoop は CreateOrUpdateStream が成功するまで指数バックオフで再試行し、成功したら
// p.ready を true にして戻る(以後は再試行しない。services/calc/cmd/calc/main.go の
// fetchMasterLoop と同じ型。ADR-0212 §5)。NATS が calc-svc より遅く Ready になっても、
// このプロセスを再起動せずに発行が有効になる(AC-N9)。
func (p *Publisher) ensureStreamLoop(ctx context.Context) {
	for attempt := 0; ; attempt++ {
		createCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := p.js.CreateOrUpdateStream(createCtx, streamConfig)
		cancel()
		if err == nil {
			p.ready.Store(true)
			return
		}
		slog.Warn("calc-svc: JetStream ストリームを作成できない。再試行する", "error", err, "attempt", attempt)
		timer := time.NewTimer(streamRetryDelay(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// streamRetryDelay は services/calc/cmd/calc/main.go の backoffDelay と同じ計算(Initial から
// 倍々に伸ばし、Max で止める。桁あふれしない)。
func streamRetryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := createStreamRetryInitial
	for i := 0; i < attempt; i++ {
		if delay >= createStreamRetryMax {
			return createStreamRetryMax
		}
		delay *= 2
		if delay <= 0 || delay > createStreamRetryMax {
			return createStreamRetryMax
		}
	}
	return delay
}

// Publish はイベントを非同期に発行する(応答をブロックしない。ADR-0212 §6)。
// ハンドラがレスポンスを書き終えた後に呼ぶこと。p が nil(発行無効)・ストリーム未作成なら
// 何もしない。detail は Operation が calcevents.OperationCalc のときだけ渡す。
func (p *Publisher) Publish(deviceID, sessionID, operation string, occurredAt time.Time, detail *calcevents.CalcDetail) {
	if p == nil || !p.ready.Load() {
		return
	}
	event := calcevents.Event{
		SchemaVersion: calcevents.SchemaVersion,
		DeviceID:      deviceID,
		SessionID:     sessionID,
		Operation:     operation,
		OccurredAt:    occurredAt,
		Detail:        detail,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		// event はこのパッケージが組み立てた構造体なので、通常は起きない。
		slog.Warn("calc-svc: イベントの JSON 化に失敗", "operation", operation, "error", err)
		return
	}
	subject := calcevents.Subject(deviceID)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		future, err := p.js.PublishAsync(subject, payload)
		if err != nil {
			slog.Warn("calc-svc: イベント発行に失敗", "operation", operation, "error", err)
			return
		}
		select {
		case <-future.Ok():
		case err := <-future.Err():
			slog.Warn("calc-svc: イベント発行に失敗", "operation", operation, "error", err)
		case <-time.After(publishTimeout):
			slog.Warn("calc-svc: イベント発行の確認がタイムアウトした", "operation", operation)
		}
	}()
}

// Shutdown はプロセス終了時に呼ぶ。発行中の goroutine と JetStream 側の未確定分を、それぞれ
// 上限付きで待ってから接続を閉じる(ADR-0212 §6。AC-N6)。http.Server.Shutdown が完了した後に
// 呼ぶこと(この関数自体は HTTP のリクエスト処理をブロックしない)。p が nil なら何もしない。
func (p *Publisher) Shutdown() {
	if p == nil {
		return
	}
	waitBounded(&p.wg, shutdownDrainTimeout)
	select {
	case <-p.js.PublishAsyncComplete():
	case <-time.After(shutdownDrainTimeout):
	}
	_ = p.nc.Drain()
}

// waitBounded は wg.Wait() を上限付きで待つ(sync.WaitGroup 自体には上限を付ける手段が無いため、
// 別 goroutine で Wait して done チャンネルの close を select で待つ。ADR-0212 §6)。
func waitBounded(wg *sync.WaitGroup, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}
