// Package events は record-svc の計算イベント消費(ADR-0209 §4・§7、ADR-0212 §6)。
//
// Handler は NATS に依存しない純粋な処理本体(consumer_test.go が固定)。NATS 固有の配線
// (JetStream への接続・durable consumer の作成・Fetch ループ)は subscriber.go に分ける。
package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"

	"example.com/pokecalc/services/internal/calcevents"
	"example.com/pokecalc/services/record/internal/store"
)

// Action は1件のメッセージの処理後にすべきこと。
type Action int

const (
	// Ack は処理済み(保存した・重複だった・墓石で捨てた)。再配送させない。
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

// DurableName は record-svc 専用の durable consumer 名(team-svc とは別。ADR-0209 §4)。
const DurableName = "record-svc"

// SubjectFilter は全端末ぶんの計算イベントを1つの consumer で受ける(ADR-0209 §4・ADR-0212 §4)。
const SubjectFilter = calcevents.SubjectPrefix + "*"

// EventID は JetStream のストリームシーケンスから重複排除キーを組み立てる。受信時刻・ランダム値を
// 混ぜない(同じメッセージの再配送では同じ値になること。ADR-0212 §6)。
//
// 注意(critic 指摘 R-9・ADR-0212 §4 実装時の追記): シーケンス番号だけに由来するため、
// CALC_EVENTS ストリームを delete して作り直すとシーケンスが1から再開し、過去の event_id と
// 衝突しうる(サイレントな Duplicate 誤認)。ストリームを作り直す運用をするときは、同じ操作で
// record DB の calc_events も空にすること(詳細は ADR-0212 §4 の追記を参照)。
func EventID(streamSeq uint64) string {
	return "calc-events-" + strconv.FormatUint(streamSeq, 10)
}

// Handler は record-svc の計算イベント消費本体。ゼロ値は使わない(NewHandler で作る)。
type Handler struct {
	store store.Store
}

// NewHandler は Store を使う Handler を作る。
func NewHandler(st store.Store) *Handler {
	return &Handler{store: st}
}

// Handle は1件のメッセージ(JetStream のストリームシーケンス由来の eventID と本文)を処理し、
// consume ループが呼ぶべき Action を返す(ADR-0209 §4・§7、ADR-0212 §6)。
func (h *Handler) Handle(ctx context.Context, eventID string, data []byte) Action {
	var ev calcevents.Event
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ev); err != nil {
		slog.Warn("record-svc: 壊れたイベント本文を破棄する", "eventId", eventID, "error", err)
		return Term
	}
	if ev.DeviceID == "" {
		slog.Warn("record-svc: 端末 ID が空のイベントを破棄する", "eventId", eventID)
		return Term
	}
	if ev.SchemaVersion != calcevents.SchemaVersion {
		slog.Warn("record-svc: 未知の schemaVersion のイベントを破棄する",
			"eventId", eventID, "schemaVersion", ev.SchemaVersion, "want", calcevents.SchemaVersion)
		return Term
	}

	payload, err := json.Marshal(ev)
	if err != nil {
		// ev はデコードに成功した構造体なので、通常は起きない。
		slog.Warn("record-svc: イベントの再 JSON 化に失敗", "eventId", eventID, "error", err)
		return Term
	}

	ce := store.CalcEvent{
		EventID:            eventID,
		DeviceID:           ev.DeviceID,
		SessionID:          ev.SessionID,
		Operation:          ev.Operation,
		OccurredAt:         ev.OccurredAt,
		DefenderSpeciesKey: defenderSpeciesKey(ev),
		Payload:            payload,
	}

	outcome, err := h.store.SaveCalcEvent(ctx, ce)
	if err != nil {
		if errors.Is(err, store.ErrUnavailable) {
			slog.Warn("record-svc: DB に届かない。再配送させる", "eventId", eventID, "error", err)
			return Nak
		}
		slog.Error("record-svc: イベント保存で想定外のエラー", "eventId", eventID, "error", err)
		return Nak
	}

	switch outcome {
	case store.Duplicate:
		slog.Debug("record-svc: 重複イベントを ack する", "eventId", eventID, "deviceId", ev.DeviceID)
	case store.Tombstoned:
		slog.Debug("record-svc: 墓石より前のイベントを破棄する", "eventId", eventID, "deviceId", ev.DeviceID)
	}
	return Ack
}

// defenderSpeciesKey は Operation が calc のときだけ防御側の種族を集計キーとして返す
// (ADR-0209 §3 #2・store.CalcEvent の docstring)。
func defenderSpeciesKey(ev calcevents.Event) string {
	if ev.Operation != calcevents.OperationCalc || ev.Detail == nil {
		return ""
	}
	return ev.Detail.Defender.SpeciesKey
}
