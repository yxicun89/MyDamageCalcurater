package calcevents

// TestEventJSONGolden は Event の JSON 表現をリテラルと比較する(ADR-0212 §7・AC-N8)。
// contract_test.go のコンパイル時チェックは、CalcDetail が丸ごと埋め込む api.Individual・
// api.FieldState・api.CalcOptions の**内部の**フィールド名がリネームされても検知できない
// (埋め込みなので構造体の代入自体は常に成立する)。ここでは実際に値を詰めて JSON 化し、
// 期待する文字列と一致するかを見ることで、埋め込み型の内部フィールド名・omitempty の有無・
// SchemaVersion まで含めて壊れたら検知する(critic レビューでの指摘)。

import (
	"encoding/json"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
)

func TestEventJSONGoldenWithDetail(t *testing.T) {
	level := 50
	abilityID := "test-ability"
	itemID := "test-item"
	moveIDOnIndividual := "test-individual-move" // Individual.MoveId(使わない方。CalcDetail.MoveID は別経路)
	teraType := api.PokeType("fire")
	status := api.StatusCondition("burn")
	critical := true
	rankAtk, rankDef, rankSpa, rankSpd, rankSpe := 1, -1, 2, -2, 0
	attacker := api.Individual{
		SpeciesKey: "0001-000",
		NatureId:   "adamant",
		Level:      &level,
		AbilityId:  &abilityID,
		ItemId:     &itemID,
		MoveId:     &moveIDOnIndividual,
		Sp:         api.StatBlock{Hp: 1, Atk: 2, Def: 3, Spa: 4, Spd: 5, Spe: 6},
		Ranks:      &api.RankBlock{Atk: &rankAtk, Def: &rankDef, Spa: &rankSpa, Spd: &rankSpd, Spe: &rankSpe},
		Status:     &status,
		TeraType:   &teraType,
	}
	defender := api.Individual{
		SpeciesKey: "0002-000",
		NatureId:   "bold",
		Sp:         api.StatBlock{Hp: 10, Atk: 20, Def: 30, Spa: 40, Spd: 50, Spe: 60},
	}
	weather := api.Weather("rain")
	terrain := api.Terrain("electric")
	attackerAurora, attackerLight, attackerReflect := true, false, true
	defenderAurora, defenderLight, defenderReflect := false, true, false
	field := &api.FieldState{
		Weather: &weather,
		Terrain: &terrain,
		AttackerScreens: &api.Screens{
			AuroraVeil: &attackerAurora, LightScreen: &attackerLight, Reflect: &attackerReflect,
		},
		DefenderScreens: &api.Screens{
			AuroraVeil: &defenderAurora, LightScreen: &defenderLight, Reflect: &defenderReflect,
		},
	}
	options := &api.CalcOptions{Critical: &critical}

	occurredAt := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	event := Event{
		SchemaVersion: SchemaVersion,
		DeviceID:      "device-1",
		SessionID:     "session-1",
		Operation:     OperationCalc,
		OccurredAt:    occurredAt,
		Detail: &CalcDetail{
			Format:            "single",
			Attacker:          attacker,
			Defender:          defender,
			MoveID:            "test-beam",
			Field:             field,
			Options:           options,
			MinPercent:        16.8,
			MaxPercent:        20.3,
			ViaRecommendation: false,
		},
	}

	got, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	const want = `{"schemaVersion":1,"deviceId":"device-1","sessionId":"session-1","operation":"calc","occurredAt":"2026-09-25T12:00:00Z",` +
		`"detail":{"format":"single",` +
		`"attacker":{"abilityId":"test-ability","itemId":"test-item","level":50,"moveId":"test-individual-move","natureId":"adamant",` +
		`"ranks":{"atk":1,"def":-1,"spa":2,"spd":-2,"spe":0},"sp":{"atk":2,"def":3,"hp":1,"spa":4,"spd":5,"spe":6},` +
		`"speciesKey":"0001-000","status":"burn","teraType":"fire"},` +
		`"defender":{"natureId":"bold","sp":{"atk":20,"def":30,"hp":10,"spa":40,"spd":50,"spe":60},"speciesKey":"0002-000"},` +
		`"moveId":"test-beam",` +
		`"field":{"attackerScreens":{"auroraVeil":true,"lightScreen":false,"reflect":true},` +
		`"defenderScreens":{"auroraVeil":false,"lightScreen":true,"reflect":false},` +
		`"terrain":"electric","weather":"rain"},` +
		`"options":{"critical":true},` +
		`"minPercent":16.8,"maxPercent":20.3,"viaRecommendation":false}}`

	if string(got) != want {
		t.Errorf("JSON が一致しない(openapi.yaml のフィールド名変更・omitempty の変化を疑う):\n got  = %s\n want = %s", got, want)
	}
}

// TestEventJSONGoldenEnvelopeOnly は calcBulk/calcReverse の envelope だけのイベント
// (Detail が無い)の JSON 表現を固定する(ADR-0212 §7.1)。
func TestEventJSONGoldenEnvelopeOnly(t *testing.T) {
	occurredAt := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	event := Event{
		SchemaVersion: SchemaVersion,
		DeviceID:      "device-1",
		SessionID:     "session-1",
		Operation:     OperationBulk,
		OccurredAt:    occurredAt,
		Detail:        nil,
	}
	got, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	const want = `{"schemaVersion":1,"deviceId":"device-1","sessionId":"session-1","operation":"calcBulk","occurredAt":"2026-09-25T12:00:00Z"}`
	if string(got) != want {
		t.Errorf("JSON が一致しない(Detail は omitempty で消えるはず):\n got  = %s\n want = %s", got, want)
	}
}
