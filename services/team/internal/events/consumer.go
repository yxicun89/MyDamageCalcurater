// Package events は team-svc の計算イベント消費(ADR-0209 §4・§7、ADR-0212 §6、ADR-0213 §5)。
//
// Handler は NATS に依存しない純粋な処理本体(consumer_test.go が固定)。NATS 固有の配線
// (JetStream への接続・durable consumer の作成・Fetch ループ)は subscriber.go に分ける。
//
// team-svc がイベントから行うのは devices.last_seen_at の更新だけで、個体・ダメージ等の中身は
// 一切読まない(ADR-0213 §5)。event_id の重複排除表は持たず、24時間規則(ADR-0209 §4)が
// そのまま冪等性になる。
package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"example.com/pokecalc/services/internal/calcevents"
	"example.com/pokecalc/services/team/internal/store"
)

// Action は1件のメッセージの処理後にすべきこと。
type Action int

const (
	// Ack は処理済み(last_seen_at を更新した・24時間以内で書かなかった・墓石で捨てた)。再配送させない。
	Ack Action = iota
	// Nak は一時的な失敗(DB に届かない)。後で再配送させる。
	Nak
	// Term は恒久的に処理できない(壊れた JSON・未知の schemaVersion)。再配送させない。
	Term
)

func (a Action) String() string {
	switch a {
	case Ack:
		return "Ack"
	case Nak:
		return "Nak"
	case Term:
		return "Term"
	default:
		return "Action(?)"
	}
}

// DurableName は team-svc 専用の durable consumer 名(record-svc とは別。ADR-0209 §4)。
const DurableName = "team-svc"

// SubjectFilter は全端末ぶんの計算イベントを1つの consumer で受ける(ADR-0209 §4・ADR-0212 §4)。
const SubjectFilter = calcevents.SubjectPrefix + "*"

// Handler は team-svc の計算イベント消費本体。ゼロ値は使わない(NewHandler で作る)。
type Handler struct {
	store store.Store
}

// NewHandler は Store を使う Handler を作る。
func NewHandler(st store.Store) *Handler {
	return &Handler{store: st}
}

// Handle は1件のメッセージ(ログ用の eventID と本文)を処理し、consume ループが呼ぶべき Action を返す
// (ADR-0209 §4・§7、ADR-0212 §6、ADR-0213 §5)。store に渡すのは端末 ID と発生時刻だけで、
// イベントの Detail(個体・ダメージ)は読み捨てる。
func (h *Handler) Handle(ctx context.Context, eventID string, data []byte) Action {
	var ev calcevents.Event
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ev); err != nil {
		slog.Warn("team-svc: 壊れたイベント本文を破棄する", "eventId", eventID, "error", err)
		return Term
	}
	if ev.DeviceID == "" {
		slog.Warn("team-svc: 端末 ID が空のイベントを破棄する", "eventId", eventID)
		return Term
	}
	if ev.SchemaVersion != calcevents.SchemaVersion {
		slog.Warn("team-svc: 未知の schemaVersion のイベントを破棄する",
			"eventId", eventID, "schemaVersion", ev.SchemaVersion, "want", calcevents.SchemaVersion)
		return Term
	}

	outcome, err := h.store.TouchDeviceFromEvent(ctx, ev.DeviceID, ev.OccurredAt)
	if err != nil {
		if errors.Is(err, store.ErrUnavailable) {
			slog.Warn("team-svc: DB に届かない。再配送させる", "eventId", eventID, "error", err)
			return Nak
		}
		slog.Error("team-svc: last_seen_at の更新で想定外のエラー", "eventId", eventID, "error", err)
		return Nak
	}

	switch outcome {
	case store.Skipped:
		slog.Debug("team-svc: 24時間以内のイベントなので last_seen_at を進めない", "eventId", eventID, "deviceId", ev.DeviceID)
	case store.Tombstoned:
		slog.Debug("team-svc: 墓石より前のイベントを破棄する", "eventId", eventID, "deviceId", ev.DeviceID)
	}
	return Ack
}
