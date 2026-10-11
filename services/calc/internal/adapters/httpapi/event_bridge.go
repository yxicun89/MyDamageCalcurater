package httpapi

// 計算イベントの発行(ADR-0212)。app のポート(app.EventPublisher)を、ワイヤフォーマット
// (services/internal/calcevents。中身に生成型を持つ)の発行口 EventPublisher へつなぐ。

import (
	"time"

	"example.com/pokecalc/services/calc/internal/app"
	"example.com/pokecalc/services/internal/calcevents"
)

// EventPublisher は計算イベントの発行(ADR-0212)。*events.Publisher が実装する
// (nil の *events.Publisher でも安全に呼べる。テストでは偽の実装に差し替える)。
// 発行は非同期で応答を一切ブロックしない(CLAUDE.md 絶対ルール5)。
type EventPublisher interface {
	Publish(deviceID, sessionID, operation string, occurredAt time.Time, detail *calcevents.CalcDetail)
}

// operationNames は app の操作をワイヤの operation に写す。
var operationNames = map[app.Operation]string{
	app.OperationCalc:    calcevents.OperationCalc,
	app.OperationBulk:    calcevents.OperationBulk,
	app.OperationReverse: calcevents.OperationReverse,
}

// eventBridge は app.EventPublisher を実装し、EventPublisher へ写して渡す。
type eventBridge struct {
	publisher EventPublisher
}

func (b eventBridge) Publish(ev app.Event) {
	var detail *calcevents.CalcDetail
	if c := ev.Calc; c != nil {
		result := calcResultFrom(c.Result)
		r := c.Request
		// 中身はリクエストのまま(技はトップレベルの moveId。ADR-0212 §7)。ViaRecommendation は常に false。
		detail = &calcevents.CalcDetail{
			Format: r.Format.Value, Attacker: individualToAPI(r.Attacker), Defender: individualToAPI(r.Defender),
			MoveID: r.MoveID, Field: fieldToAPI(r.Field), Options: optionsToAPI(r.Options),
			BattleState: eventBattleState(battleStateToAPI(r.BattleState)),
			MinPercent:  result.MinPercent, MaxPercent: result.MaxPercent,
		}
	}
	b.publisher.Publish(ev.Meta.DeviceID, ev.Meta.SessionID, operationNames[ev.Operation], ev.OccurredAt, detail)
}
