// NATS JetStream への配線(pull consumer)。Handler(consumer.go。NATS に依存しない)を、
// 実際の JetStream の durable consumer に繋ぐ。record-svc の障害は自分の中に収め、NATS に
// 接続できない・consumer を作れなくてもプロセスの起動を妨げない(CLAUDE.md 絶対ルール5。
// services/calc/internal/events/events.go の Publisher と同じ流儀)。
package events

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"example.com/pokecalc/services/internal/calcevents"
)

const (
	// connectTimeout は初回接続の試行1回ぶんの上限(calc-svc の Publisher と同じ理由)。
	connectTimeout = 500 * time.Millisecond

	// createConsumerRetryInitial・createConsumerRetryMax は durable consumer 作成の再試行間隔
	// (ストリームが calc-svc よりまだ無い場合に備える。calc-svc の streamRetryDelay と同じ型)。
	createConsumerRetryInitial = 500 * time.Millisecond
	createConsumerRetryMax     = 30 * time.Second

	// ackWait はサーバーが ack を待つ上限(これを超えると再配送される)。
	ackWait = 30 * time.Second

	// handleTimeout は1件のメッセージの Handle(DB アクセスを含む)に与える上限(critic 指摘 R-8。
	// ackWait より短くして、Nak を返せないまま応答不能になるより先にこちらで打ち切る)。
	handleTimeout = 20 * time.Second

	// shutdownDrainTimeout は終了時、処理中のメッセージを待つ上限。
	shutdownDrainTimeout = 5 * time.Second
)

// consumerConfig は record-svc の durable consumer の設定(ADR-0209 §4)。
var consumerConfig = jetstream.ConsumerConfig{
	Durable:       DurableName,
	FilterSubject: SubjectFilter,
	AckPolicy:     jetstream.AckExplicitPolicy,
	DeliverPolicy: jetstream.DeliverAllPolicy,
	AckWait:       ackWait,
	MaxDeliver:    -1, // 恒久的な失敗(Term)以外は消費できるまで再配送させ続ける
}

// Subscriber は record-svc から NATS JetStream(CALC_EVENTS ストリーム)への購読を1つにまとめる。
// ゼロ値は使わない(New で作る)。nil の *Subscriber への呼び出しも安全(何もしない)。
type Subscriber struct {
	nc      *nats.Conn
	js      jetstream.JetStream
	handler *Handler

	mu         sync.Mutex
	consumeCtx jetstream.ConsumeContext
}

// New は natsURL(空なら購読を丸ごと無効化)から Subscriber を作る。NATS に接続できない・
// consumer を作成できなくても、この関数自体は失敗しない(呼び出し元の起動をブロックしない。
// CLAUDE.md 絶対ルール5)。
func New(natsURL string, handler *Handler) *Subscriber {
	if natsURL == "" {
		return nil
	}
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.Timeout(connectTimeout),
	)
	if err != nil {
		slog.Warn("record-svc: NATS に接続できない。購読を無効化する", "error", err)
		return nil
	}
	js, err := jetstream.New(nc)
	if err != nil {
		slog.Warn("record-svc: JetStream を初期化できない。購読を無効化する", "error", err)
		nc.Close()
		return nil
	}

	s := &Subscriber{nc: nc, js: js, handler: handler}
	go s.ensureConsumeLoop(context.Background())
	return s
}

// ensureConsumeLoop は durable consumer の作成 → Consume の開始が成功するまで指数バックオフで
// 再試行する(calc-svc の Publisher.ensureStreamLoop と同じ型)。ストリームが calc-svc 側でまだ
// 作られていなくても、record-svc の起動を妨げない。
func (s *Subscriber) ensureConsumeLoop(ctx context.Context) {
	for attempt := 0; ; attempt++ {
		if s.tryStartConsuming(ctx) {
			return
		}
		timer := time.NewTimer(retryDelay(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Subscriber) tryStartConsuming(ctx context.Context) bool {
	createCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	consumer, err := s.js.CreateOrUpdateConsumer(createCtx, calcevents.StreamName, consumerConfig)
	if err != nil {
		slog.Warn("record-svc: durable consumer を用意できない。再試行する", "error", err)
		return false
	}

	consumeCtx, err := consumer.Consume(s.onMessage, jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
		slog.Warn("record-svc: 購読でエラーが発生した", "error", err)
	}))
	if err != nil {
		slog.Warn("record-svc: 購読を開始できない。再試行する", "error", err)
		return false
	}

	s.mu.Lock()
	s.consumeCtx = consumeCtx
	s.mu.Unlock()
	return true
}

// onMessage は1件の JetStream メッセージを Handler.Handle に渡し、戻り値の Action に従って
// Ack / Nak / Term を呼ぶ(NATS 固有の処理はここだけに閉じる。consumer.go はテスト可能な純粋関数のまま)。
func (s *Subscriber) onMessage(msg jetstream.Msg) {
	meta, err := msg.Metadata()
	if err != nil {
		slog.Error("record-svc: メッセージのメタデータを読めない。再配送させる", "error", err)
		if nakErr := msg.Nak(); nakErr != nil {
			slog.Warn("record-svc: Nak に失敗", "error", nakErr)
		}
		return
	}

	// critic 指摘 R-8: DB がハングしても1メッセージの処理が無期限に詰まらないよう、AckWait より
	// 短い上限を付ける(この上限を超えたら Nak 相当のまま応答が無くなり、AckWait 経過後にサーバーが
	// 自動で再配送する)。
	ctx, cancel := context.WithTimeout(context.Background(), handleTimeout)
	defer cancel()

	id := EventID(meta.Sequence.Stream)
	action := s.handler.Handle(ctx, id, msg.Data())

	var ackErr error
	switch action {
	case Ack:
		ackErr = msg.Ack()
	case Nak:
		ackErr = msg.Nak()
	case Term:
		ackErr = msg.Term()
	}
	if ackErr != nil {
		slog.Warn("record-svc: メッセージへの応答に失敗", "eventId", id, "action", action.String(), "error", ackErr)
	}
}

// retryDelay は calc-svc の streamRetryDelay と同じ計算(Initial から倍々に伸ばし、Max で止める)。
func retryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := createConsumerRetryInitial
	for i := 0; i < attempt; i++ {
		if delay >= createConsumerRetryMax {
			return createConsumerRetryMax
		}
		delay *= 2
		if delay <= 0 || delay > createConsumerRetryMax {
			return createConsumerRetryMax
		}
	}
	return delay
}

// Shutdown はプロセス終了時に呼ぶ。処理中のメッセージを上限付きで待ってから接続を閉じる。
// s が nil なら何もしない。
func (s *Subscriber) Shutdown() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cc := s.consumeCtx
	s.mu.Unlock()
	if cc != nil {
		cc.Drain()
	}
	done := make(chan struct{})
	go func() {
		_ = s.nc.Drain()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownDrainTimeout):
	}
}
