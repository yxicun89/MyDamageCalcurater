package app

// app のユースケースを HTTP なしで、偽のポート(Master・EventPublisher・Expired)で確かめる(ADR-0811)。
// 契約どおりの応答(ステータス・本文)は adapters/httpapi のテストが固定する。ここは手順の順番と発行の有無を見る。

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/domain"
)

const (
	speciesA   = "9001-000"
	speciesB   = "9002-000"
	moveTackle = "tackle"
	natureN    = "neutral"
)

// fakeMaster は架空の 2 種族・1 技・1 性格だけを持つ Master。
type fakeMaster struct {
	chart engine.TypeChart
}

func newFakeMaster(t *testing.T) fakeMaster {
	t.Helper()
	chart, err := engine.NewTypeChart(engine.TypeChartData{Types: []engine.Type{engine.TypeNormal, engine.TypeWater}})
	if err != nil {
		t.Fatalf("相性表を作れない: %v", err)
	}
	return fakeMaster{chart: chart}
}

func (fakeMaster) Species(key string) (engine.Species, bool) {
	stats := engine.Stats{HP: 80, Atk: 80, Def: 80, SpA: 80, SpD: 80, Spe: 80}
	switch key {
	case speciesA:
		return engine.Species{Key: key, Types: []engine.Type{engine.TypeNormal}, BaseStats: stats}, true
	case speciesB:
		return engine.Species{Key: key, Types: []engine.Type{engine.TypeWater}, BaseStats: stats}, true
	}
	return engine.Species{}, false
}

func (fakeMaster) Move(id string) (engine.Move, bool) {
	if id != moveTackle {
		return engine.Move{}, false
	}
	return engine.Move{ID: id, Type: engine.TypeNormal, Category: engine.CategoryPhysical, Power: 40}, true
}

func (fakeMaster) Item(string) (engine.Item, bool)       { return engine.Item{}, false }
func (fakeMaster) Ability(string) (engine.Ability, bool) { return engine.Ability{}, false }

func (fakeMaster) Nature(id string) (engine.Nature, bool) {
	return engine.NatureNeutral, id == natureN
}

func (fakeMaster) NatureID(n engine.Nature) (string, bool) {
	return natureN, n == engine.NatureNeutral
}

func (fakeMaster) MegaRequiredItem(string) (string, bool) { return "", false }
func (m fakeMaster) TypeChart() engine.TypeChart          { return m.chart }

type recordingPublisher struct{ events []Event }

func (p *recordingPublisher) Publish(ev Event) { p.events = append(p.events, ev) }

func known(v string) *domain.Enum   { return &domain.Enum{Value: v, Known: true} }
func unknown(v string) *domain.Enum { return &domain.Enum{Value: v, Known: false} }

func validCalc() CalcRequest {
	return CalcRequest{
		Format:   domain.Enum{Value: "single", Known: true},
		Attacker: Individual{SpeciesKey: speciesA, NatureID: natureN},
		Defender: Individual{SpeciesKey: speciesB, NatureID: natureN},
		MoveID:   moveTackle,
	}
}

func codeOf(t *testing.T, err error) domain.Code {
	t.Helper()
	var de *domain.Error
	if !errors.As(err, &de) {
		t.Fatalf("err = %v(%T), want *domain.Error", err, err)
	}
	return de.Code
}

// 成功した計算は Detail(リクエストのまま + 結果)入りのイベントを 1 件だけ発行する(ADR-0212 §7)。
func TestCalcPublishesOneEventWithRequest(t *testing.T) {
	pub := &recordingPublisher{}
	svc := New(newFakeMaster(t), pub, nil)
	meta := Meta{DeviceID: "device-1", SessionID: "session-1"}
	req := validCalc()

	before := time.Now()
	res, err := svc.Calc(context.Background(), meta, req)
	if err != nil {
		t.Fatalf("Calc: %v", err)
	}
	if res.MaxDamage() <= 0 {
		t.Errorf("ダメージが出ていない: %+v", res)
	}
	if len(pub.events) != 1 {
		t.Fatalf("発行回数 = %d, want 1", len(pub.events))
	}
	ev := pub.events[0]
	if ev.Meta != meta || ev.Operation != OperationCalc || ev.Calc == nil {
		t.Fatalf("イベントが違う: %+v", ev)
	}
	if ev.Calc.Request.MoveID != moveTackle || ev.Calc.Result.MaxDamage() != res.MaxDamage() {
		t.Errorf("Detail がリクエスト・結果のままでない: %+v", ev.Calc)
	}
	if ev.OccurredAt.Location() != time.UTC || ev.OccurredAt.Before(before.Add(-time.Second)) {
		t.Errorf("OccurredAt = %v, want 計算した時刻の UTC", ev.OccurredAt)
	}
}

// 失敗の順番は HTTP 境界にあったときと同じ(ADR-0200。同じ入力は同じ code)。失敗したら発行しない。
func TestCalcFailureOrder(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CalcRequest)
		want   domain.Code
	}{
		{"format が先", func(r *CalcRequest) {
			r.Format = domain.Enum{Value: "triple"}
			r.Attacker.SpeciesKey = "0000-000"
		}, domain.CodeInvalidEnum},
		{"攻撃側の ID は防御側の列挙より先", func(r *CalcRequest) {
			r.Attacker.SpeciesKey = "0000-000"
			r.Defender.Status = unknown("sleepy")
		}, domain.CodeUnknownSpecies},
		{"個体の列挙は同じ個体の ID より先", func(r *CalcRequest) {
			r.Attacker.SpeciesKey = "0000-000"
			r.Attacker.TeraType = unknown("cosmic")
		}, domain.CodeInvalidEnum},
		{"技の ID は場の列挙より先", func(r *CalcRequest) {
			r.MoveID = "nope"
			r.Field = &domain.Field{Weather: unknown("snowstorm")}
		}, domain.CodeUnknownMove},
		{"場の列挙は対戦の状態より先", func(r *CalcRequest) {
			r.Field = &domain.Field{Terrain: unknown("lava")}
			zero := 0
			r.BattleState = &domain.BattleState{Hits: &zero}
		}, domain.CodeInvalidEnum},
		{"対戦の状態の 0 は invalid_input", func(r *CalcRequest) {
			zero := 0
			r.BattleState = &domain.BattleState{DefenderCurrentHP: &zero}
		}, domain.CodeInvalidInput},
		{"既知の列挙は通る", func(r *CalcRequest) {
			r.Field = &domain.Field{Weather: known("rain")}
			r.MoveID = "nope"
		}, domain.CodeUnknownMove},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub := &recordingPublisher{}
			req := validCalc()
			tt.mutate(&req)
			_, err := New(newFakeMaster(t), pub, nil).Calc(context.Background(), Meta{}, req)
			if got := codeOf(t, err); got != tt.want {
				t.Errorf("code = %q, want %q(err=%v)", got, tt.want, err)
			}
			if len(pub.events) != 0 {
				t.Errorf("失敗したのに発行した: %+v", pub.events)
			}
		})
	}
}

// 締め切りは入力の検証の後・engine の前に見る(ADR-0801)。過ぎていれば計算も発行もしない。
func TestDeadlineCheckedAfterValidation(t *testing.T) {
	expired := func(context.Context) bool { return true }
	pub := &recordingPublisher{}
	svc := New(newFakeMaster(t), pub, expired)

	if _, err := svc.Calc(context.Background(), Meta{}, validCalc()); !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("Calc の err = %v, want ErrDeadlineExceeded", err)
	}
	bad := validCalc()
	bad.MoveID = "nope"
	if _, err := svc.Calc(context.Background(), Meta{}, bad); codeOf(t, err) != domain.CodeUnknownMove {
		t.Errorf("検証の失敗が締め切りより後になった: %v", err)
	}
	if _, err := svc.Bulk(context.Background(), Meta{}, BulkRequest{
		Format: domain.Enum{Value: "single", Known: true}, Attacker: validCalc().Attacker,
		DefenderSpeciesKey: speciesB, MoveID: moveTackle,
	}); !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("Bulk の err = %v, want ErrDeadlineExceeded", err)
	}
	if len(pub.events) != 0 {
		t.Errorf("締め切りを過ぎたのに発行した: %+v", pub.events)
	}
}

// 一括計算は envelope だけを 1 件発行し、行ごとの性格 ID を Master から引く(ADR-0212 §7.1・ADR-0200)。
func TestBulkPublishesEnvelopeAndNatureIDs(t *testing.T) {
	pub := &recordingPublisher{}
	out, err := New(newFakeMaster(t), pub, nil).Bulk(context.Background(), Meta{DeviceID: "d"}, BulkRequest{
		Format: domain.Enum{Value: "single", Known: true}, Attacker: validCalc().Attacker,
		DefenderSpeciesKey: speciesB, MoveID: moveTackle,
	})
	if err != nil {
		t.Fatalf("Bulk: %v", err)
	}
	if len(out.Result.Rows) == 0 || len(out.NatureIDs) != len(out.Result.Rows) {
		t.Fatalf("行と性格 ID の数が合わない: rows=%d natureIDs=%d", len(out.Result.Rows), len(out.NatureIDs))
	}
	if len(pub.events) != 1 || pub.events[0].Operation != OperationBulk || pub.events[0].Calc != nil {
		t.Errorf("envelope だけを 1 件のはず: %+v", pub.events)
	}
}

// reverseBase は観測を持たない逆算のリクエスト(観測は各テストで足す)。
func reverseBase() ReverseRequest {
	return ReverseRequest{
		Format: domain.Enum{Value: "single", Known: true}, Side: engine.ReverseSide("defender"),
		Known: validCalc().Attacker, UnknownSpeciesKey: speciesB, MoveID: moveTackle,
	}
}

// 逆算の観測の検証は、持ち物候補の解決より後・個体の検証(ValidateIndividual)より前に行う。
// 観測が無ければ no_observation、持ち物候補が解決できなければ観測が無くても unknown_item、
// 個体が不正でも観測が無ければ no_observation(観測があれば個体の invalid_input)。
func TestReverseObservationOrder(t *testing.T) {
	svc := New(newFakeMaster(t), nil, nil)

	if _, err := svc.Reverse(context.Background(), Meta{}, reverseBase()); codeOf(t, err) != domain.CodeNoObservation {
		t.Errorf("観測なし: %v", err)
	}

	badItem := reverseBase()
	missing := "no-such-item"
	badItem.ItemCandidates = &[]*string{&missing}
	if _, err := svc.Reverse(context.Background(), Meta{}, badItem); codeOf(t, err) != domain.CodeUnknownItem {
		t.Errorf("持ち物候補の解決が観測の検証より後になった: %v", err)
	}

	badIndividual := reverseBase()
	badIndividual.Known.SP = engine.Stats{Atk: engine.MaxSPPerStat + 1}
	if _, err := svc.Reverse(context.Background(), Meta{}, badIndividual); codeOf(t, err) != domain.CodeNoObservation {
		t.Errorf("個体の検証が観測の検証より先になった: %v", err)
	}
	damage := 10
	badIndividual.Observations = []domain.Observation{{Damage: &damage}}
	if _, err := svc.Reverse(context.Background(), Meta{}, badIndividual); codeOf(t, err) != domain.CodeInvalidInput {
		t.Errorf("観測が妥当なら個体の不正は invalid_input のはず: %v", err)
	}
}

// 逆算も締め切りを入力の検証の後・engine の前に見る(ADR-0801)。過ぎていれば探索も発行もしない。
func TestReverseDeadlineCheckedAfterValidation(t *testing.T) {
	expired := func(context.Context) bool { return true }
	pub := &recordingPublisher{}
	svc := New(newFakeMaster(t), pub, expired)

	damage := 10
	req := reverseBase()
	req.Observations = []domain.Observation{{Damage: &damage}}
	if _, err := svc.Reverse(context.Background(), Meta{}, req); !errors.Is(err, ErrDeadlineExceeded) {
		t.Errorf("Reverse の err = %v, want ErrDeadlineExceeded", err)
	}
	if _, err := svc.Reverse(context.Background(), Meta{}, reverseBase()); codeOf(t, err) != domain.CodeNoObservation {
		t.Errorf("検証の失敗が締め切りより後になった: %v", err)
	}
	if len(pub.events) != 0 {
		t.Errorf("締め切りを過ぎたのに発行した: %+v", pub.events)
	}
}
