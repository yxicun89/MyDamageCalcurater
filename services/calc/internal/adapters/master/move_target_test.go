package master

// 技の対象(MasterMove.target。issue 288・ADR-0223)の取り込みのテスト。
//
//   - FromExport は MasterMove.target(Showdown の文字列)を共通マスタ経由で engine.Move.Target に写す:
//     全体技(allAdjacent・allAdjacentFoes)→ spread、その他の既知の値 → single、null → ""(不明)。
//   - 未知の値は ErrInvalidMaster(黙って single や不明にしない)。
//   - target キーの無い本文(target を運ばない古い pokedex-svc)も受け付け、不明として扱う(入れ替えの順序に依存しない)。
//   - 写した対象で、ダブルの攻撃技の印 move_target_unknown が「対象が不明な技だけ」に付く(engine の挙動は ADR-0222)。

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// exportWithTargets は baseExport の技に対象を足したもの。testbeam = 単体技、testwave = 全体技、
// testunknown = 対象が不明(null)の攻撃技。
func exportWithTargets(t *testing.T) api.MasterExport {
	t.Helper()
	ex := baseExport(t)
	for i := range ex.Moves {
		switch ex.Moves[i].Id {
		case "testbeam":
			ex.Moves[i].Target = strPtr("normal")
		case "testwave":
			ex.Moves[i].Target = strPtr("allAdjacentFoes")
		}
	}
	ex.Moves = append(ex.Moves, api.MasterMove{
		Id: "testunknown", NameJa: "テストふめい", Type: api.PokeTypeNormal, Category: api.Physical, Power: 80,
		Mechanisms: []string{}, Target: nil,
	})
	return ex
}

// AC-T2: FromExport が対象を engine.Move.Target に写す。
func TestFromExportMapsMoveTarget(t *testing.T) {
	store := newStore(t, exportWithTargets(t))
	cases := []struct {
		id   string
		want engine.MoveTarget
	}{
		{"testbeam", engine.MoveTargetSingle},
		{"testwave", engine.MoveTargetSpread},
		{"testunknown", ""},
	}
	for _, tc := range cases {
		mv, ok := store.Move(tc.id)
		if !ok {
			t.Fatalf("Move(%s) が見つからない", tc.id)
		}
		if mv.Target != tc.want {
			t.Errorf("Move(%s).Target = %q, want %q", tc.id, mv.Target, tc.want)
		}
	}

	// 自分・場の技など、全体技でない既知の値はすべて single(ダブルの全体の補正をかけない)。
	for _, raw := range []string{"self", "allySide", "foeSide", "all", "any", "randomNormal", "adjacentAlly"} {
		ex := exportWithTargets(t)
		ex.Moves[0].Target = strPtr(raw)
		mv, ok := newStore(t, ex).Move(ex.Moves[0].Id)
		if !ok || mv.Target != engine.MoveTargetSingle {
			t.Errorf("target=%q: Target = %q, want single", raw, mv.Target)
		}
	}
}

// AC-T2: 未知の対象は ErrInvalidMaster。
func TestFromExportRejectsUnknownMoveTarget(t *testing.T) {
	for _, raw := range []string{"teleport", "AllAdjacentFoes", "spread", "single", ""} {
		t.Run(raw, func(t *testing.T) {
			ex := exportWithTargets(t)
			ex.Moves[0].Target = strPtr(raw)
			_, err := FromExport(ex)
			if raw == "" {
				// 空文字列は null と同じ「不明」(共通マスタの MoveTargetOf と同じ扱い)。
				if err != nil {
					t.Fatalf("target=\"\": FromExport = %v, want nil(不明として通す)", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidMaster) {
				t.Fatalf("target=%q: FromExport = %v, want ErrInvalidMaster", raw, err)
			}
		})
	}
}

// AC-T3: target キーの無い本文(古い pokedex-svc)も DecodeExport・FromExport を通り、対象は不明になる。
// target が null の本文も同じ。例のファイルの対象(testbeam = normal・testwave = allAdjacentFoes)は写る。
func TestDecodeExportMoveTargetCompat(t *testing.T) {
	example := string(readExample(t))
	if !strings.Contains(example, `"target": "allAdjacentFoes"`) {
		t.Fatal(`例のファイルに "target": "allAdjacentFoes" が無い(テストの前提)`)
	}

	ex, err := DecodeExport(strings.NewReader(example))
	if err != nil {
		t.Fatalf("DecodeExport(例) = %v", err)
	}
	store := newStore(t, ex)
	if mv, _ := store.Move("testwave"); mv.Target != engine.MoveTargetSpread {
		t.Errorf("例の testwave.Target = %q, want spread", mv.Target)
	}
	if mv, _ := store.Move("testbeam"); mv.Target != engine.MoveTargetSingle {
		t.Errorf("例の testbeam.Target = %q, want single", mv.Target)
	}
	if mv, _ := store.Move("testglare"); mv.Target != "" {
		t.Errorf("例の testglare.Target = %q, want \"\"(null は不明)", mv.Target)
	}

	for name, body := range map[string]string{
		"target キーが無い(古い pokedex-svc)": strings.NewReplacer(
			`, "target": "normal"`, ``, `, "target": "allAdjacentFoes"`, ``, `, "target": null`, ``).Replace(example),
		"target が null": strings.NewReplacer(
			`"target": "normal"`, `"target": null`, `"target": "allAdjacentFoes"`, `"target": null`).Replace(example),
	} {
		t.Run(name, func(t *testing.T) {
			if body == example {
				t.Fatal("置換が効いていない(テストの前提が崩れた)")
			}
			ex, err := DecodeExport(strings.NewReader(body))
			if err != nil {
				t.Fatalf("DecodeExport = %v, want nil(対象が無くても取り込める)", err)
			}
			store := newStore(t, ex)
			for _, id := range []string{"testbeam", "testwave", "testglare"} {
				if mv, _ := store.Move(id); mv.Target != "" {
					t.Errorf("%s.Target = %q, want \"\"(不明)", id, mv.Target)
				}
			}
		})
	}
}

// AC-T6: マスタから写した対象で、ダブルの攻撃技の印 move_target_unknown は対象が不明な技だけに付く。
// 対象が分かる単体技・全体技には付かない。全体技はダブルで単体技より小さい(×3072/4096)。シングルでは印は付かない。
func TestMasterMoveTargetDrivesDoubleMarks(t *testing.T) {
	store := newStore(t, exportWithTargets(t))
	attacker, ok := store.Species("9001-000")
	if !ok {
		t.Fatal("攻撃側 9001-000 が無い")
	}
	defender, ok := store.Species("9002-000")
	if !ok {
		t.Fatal("防御側 9002-000 が無い")
	}
	chart := store.TypeChart()

	calc := func(t *testing.T, moveID string, format engine.Format) engine.DamageResult {
		t.Helper()
		mv, ok := store.Move(moveID)
		if !ok {
			t.Fatalf("技 %s が無い", moveID)
		}
		in := engine.DamageInput{
			Format:    format,
			Attacker:  engine.Individual{Species: attacker, Level: 50},
			Defender:  engine.Individual{Species: defender, Level: 50},
			Move:      mv,
			TypeChart: chart,
		}
		res, err := engine.CalcDamage(in)
		if err != nil {
			t.Fatalf("CalcDamage(%s, %s): %v", moveID, format, err)
		}
		return res
	}
	hasUnknownMark := func(res engine.DamageResult) bool {
		for _, m := range res.Unsupported {
			if m.Reason == engine.UnsupportedMoveTargetUnknown {
				return true
			}
		}
		return false
	}

	for _, tc := range []struct {
		id       string
		wantMark bool
	}{
		{"testbeam", false},
		{"testwave", false},
		{"testunknown", true},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if got := hasUnknownMark(calc(t, tc.id, engine.FormatDouble)); got != tc.wantMark {
				t.Errorf("double: move_target_unknown の印 = %v, want %v", got, tc.wantMark)
			}
			if hasUnknownMark(calc(t, tc.id, engine.FormatSingle)) {
				t.Errorf("single: move_target_unknown の印が付いた(シングルでは付けない)")
			}
		})
	}

	// 全体技はダブルでだけ小さくなる(シングルの結果は対象の有無で変わらない = ゴールデン不変)。
	sWave, dWave := calc(t, "testwave", engine.FormatSingle), calc(t, "testwave", engine.FormatDouble)
	if !(dWave.MaxDamage() < sWave.MaxDamage()) {
		t.Errorf("全体技: double=%d single=%d(double < single のはず)", dWave.MaxDamage(), sWave.MaxDamage())
	}
	unknownSingle := exportWithTargets(t)
	unknownSingle.Moves[1].Target = nil // testwave の対象を消す
	sNoTarget := func() engine.DamageResult {
		st := newStore(t, unknownSingle)
		mv, _ := st.Move("testwave")
		res, err := engine.CalcDamage(engine.DamageInput{Format: engine.FormatSingle,
			Attacker: engine.Individual{Species: attacker, Level: 50}, Defender: engine.Individual{Species: defender, Level: 50},
			Move: mv, TypeChart: chart})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}()
	if !reflect.DeepEqual(sNoTarget.Rolls, sWave.Rolls) {
		t.Errorf("シングルの結果が対象の有無で変わった: %v vs %v", sNoTarget.Rolls, sWave.Rolls)
	}
}
