package importer_test

// 技の対象(moves.target。ADR-0136・issue #288)の取り込みと照合。
//
// 取得元: Showdown の技データの `target`(全技が持つ。値は取得元の文字列のまま)。
// 照合: @smogon/calc 0.12.0 の技データは全体技(allAdjacent・allAdjacentFoes)にだけ target を持ち、
// それ以外は省略する(fetch-calc は省略を "" で出す)。実データ(Champions 世代)では calc が target を持つ
// 39 技はすべて Showdown と同じ値、省略した技に Showdown の全体技は無い(ADR-0136「調査」)。
// この関係が崩れたら、ダブルの全体技補正(判定レーン)が oracle とずれるので、攻撃技は取り込みを止める。
//
// 架空データ(testdata/fictional)が持つ形:
//
//	testflame   Showdown allAdjacentFoes / calc allAdjacentFoes(両方にある攻撃技・全体技)
//	teststrike  Showdown normal          / calc ""(両方にある攻撃技・単体)
//	testglare   Showdown normal          / calc ""(両方にある変化技)
//	testrevived Showdown allAdjacent     / calc は断片(Showdown の値で補う技)
//	testsplash  Showdown randomNormal    / calc に無い(Showdown の値で補う技)
//	testold・testbanned は moves 表に採らない技

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func targetPtr(s string) *string { return &s }

func moveTargetsByID(out importer.Output) map[string]string {
	m := map[string]string{}
	for _, r := range out.Moves {
		m[r.ID] = r.Target
	}
	return m
}

// hasTargetFinding は Detail が "target" の値の食い違いがあるか。
func hasTargetFinding(findings []importer.Finding, id string) bool {
	for _, f := range findings {
		if f.Kind == importer.KindMoveValueMismatch && f.ID == id && f.Detail == "target" {
			return true
		}
	}
	return false
}

func TestConvertMoveTargetsFromFixture(t *testing.T) {
	out, rep := convertOK(t, loadFixture(t))
	want := map[string]string{
		"testflame":   "allAdjacentFoes",
		"teststrike":  "normal",
		"testglare":   "normal",
		"testrevived": "allAdjacent",
		"testsplash":  "randomNormal",
	}
	if got := moveTargetsByID(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("技の対象 = %v,\nwant %v", got, want)
	}
	for _, f := range append(append([]importer.Finding{}, rep.Warnings...), rep.Blockers...) {
		if f.Kind == importer.KindMoveValueMismatch && f.Detail == "target" {
			t.Errorf("fixture は calc と Showdown の対象が一致しているのに食い違いが出た: %+v", f)
		}
	}
}

// TestConvertMoveTargetMatchesCalc は calc と Showdown の対象の照合。
//   - calc が target を持つなら Showdown と同じ値であること
//   - calc が省略("")なら Showdown は全体技(allAdjacent・allAdjacentFoes)でないこと
//
// 食い違いは攻撃技なら Blocker(ダブルのダメージに効く)、変化技なら警告(タイプの食い違いと同じ扱い。
// ADR-0002 追記 P2-1c 規則3)。どちらも Kind は move-value-mismatch・Detail は "target"。
func TestConvertMoveTargetMatchesCalc(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(t *testing.T, in *importer.Input)
		id      string
		blocked bool // true: Blocker で止まる / false: 警告だけで取り込む
	}{
		{"calc が全体技を省略・Showdown は全体技(攻撃技)", func(t *testing.T, in *importer.Input) {
			calcMove(t, in, "Test Flame").Target = targetPtr("")
		}, "testflame", true},
		{"calc と Showdown で全体技の種類が違う(攻撃技)", func(t *testing.T, in *importer.Input) {
			showdownMove(t, in, "testflame").Target = targetPtr("allAdjacent")
		}, "testflame", true},
		{"calc は全体技・Showdown は単体(攻撃技)", func(t *testing.T, in *importer.Input) {
			calcMove(t, in, "Test Strike").Target = targetPtr("allAdjacentFoes")
		}, "teststrike", true},
		{"calc は省略・Showdown は全体技(変化技)", func(t *testing.T, in *importer.Input) {
			showdownMove(t, in, "testglare").Target = targetPtr("allAdjacentFoes")
		}, "testglare", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			tc.mutate(t, &in)
			out, rep, err := importer.Convert(in)
			if tc.blocked {
				if !errors.Is(err, importer.ErrBlocked) {
					t.Fatalf("err = %v, want ErrBlocked", err)
				}
				if !hasTargetFinding(rep.Blockers, tc.id) {
					t.Errorf("Blockers に move-value-mismatch/%s(target)が無い: %+v", tc.id, rep.Blockers)
				}
				if !reflect.DeepEqual(out, importer.Output{}) {
					t.Errorf("止めたのに Output が空でない(部分的な結果を投入に回さない)")
				}
				return
			}
			if err != nil {
				t.Fatalf("Convert: %v(変化技の対象の食い違いで止めない)", err)
			}
			if !hasTargetFinding(rep.Warnings, tc.id) {
				t.Errorf("Warnings に move-value-mismatch/%s(target)が無い: %+v", tc.id, rep.Warnings)
			}
			if hasTargetFinding(rep.Blockers, tc.id) {
				t.Errorf("変化技の対象の食い違いが Blockers にある: %+v", rep.Blockers)
			}
			// 値は Showdown を採る(calc は全体技以外を省略するので、Showdown が対象の正)。
			if got := moveTargetsByID(out)[tc.id]; got != "allAdjacentFoes" {
				t.Errorf("%s の対象 = %q, want Showdown の値", tc.id, got)
			}
		})
	}
}

// TestConvertMoveTargetNonSpreadNeedsNoCalcTarget は calc が省略した技で、Showdown が全体技以外の
// どの値でも食い違いにしないこと(calc は単体・自分・場 等を省略する)。
func TestConvertMoveTargetNonSpreadNeedsNoCalcTarget(t *testing.T) {
	for _, target := range []string{"normal", "self", "any", "adjacentFoe", "all", "foeSide", "randomNormal", "scripted"} {
		t.Run(target, func(t *testing.T) {
			in := loadFixture(t)
			showdownMove(t, &in, "teststrike").Target = targetPtr(target)
			out, rep := convertOK(t, in)
			if hasTargetFinding(rep.Warnings, "teststrike") {
				t.Errorf("食い違いでないのに警告がある: %+v", rep.Warnings)
			}
			if got := moveTargetsByID(out)["teststrike"]; got != target {
				t.Errorf("teststrike の対象 = %q, want %q", got, target)
			}
		})
	}
}

// TestConvertRejectsBadMoveTarget は取得元の対象が無い・未知の値なら ErrInvalidData で止めること
// (黙って既定の対象にしない。ADR-0115。未知の値は DB の CHECK でも拒否されるので、変換の段階で原因を示す)。
// 両方にある技と Showdown だけの技の両方の経路を確かめる。
func TestConvertRejectsBadMoveTarget(t *testing.T) {
	cases := []struct {
		name   string
		id     string
		target *string
	}{
		{"未知の値(両方にある技)", "teststrike", targetPtr("teleport")},
		{"大文字違い(両方にある技)", "teststrike", targetPtr("Normal")},
		{"空(両方にある技)", "teststrike", targetPtr("")},
		{"無い(両方にある技)", "teststrike", nil},
		{"未知の値(Showdown だけの技)", "testsplash", targetPtr("teleport")},
		{"無い(Showdown だけの技)", "testsplash", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			showdownMove(t, &in, tc.id).Target = tc.target
			if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
				t.Fatalf("err = %v, want ErrInvalidData", err)
			}
		})
	}
}

// TestConvertRejectsCalcMoveWithoutTarget は calc の技の target が無い(Input を直接組んだ・古い取得物)なら
// ErrInvalidData で止めること(省略 "" と「無い」を区別する。無いまま照合すると全体技がすべて食い違いになり、
// 原因が分かりにくい)。
func TestConvertRejectsCalcMoveWithoutTarget(t *testing.T) {
	in := loadFixture(t)
	calcMove(t, &in, "Test Strike").Target = nil
	if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
		t.Fatalf("err = %v, want ErrInvalidData", err)
	}
}

// 対象を持たない古いスナップショットはデコードで拒否する(ADR-0121 の mechanism と同じ扱い)。
// Showdown も calc も、取り直し(`make import-fetch`)を促すエラーにする。
func TestDecodeSnapshotsRequireMoveTarget(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		decode func([]byte) error
	}{
		{"showdown", filepath.Join(fixtureRoot, "generated", "showdown", "abad1deaabad1deaabad1deaabad1deaabad1dea", "snapshot.json"),
			func(raw []byte) error { _, err := importer.DecodeShowdownSnapshot(raw); return err }},
		{"calc", filepath.Join(fixtureRoot, "generated", "calc", "test-calc-1", "snapshot.json"),
			func(raw []byte) error { _, err := importer.DecodeCalcSnapshot(raw); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.decode(raw); err != nil {
				t.Fatalf("fixture のデコードに失敗: %v", err)
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			moves := doc["moves"].([]any)
			delete(moves[0].(map[string]any), "target")
			old, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			err = tc.decode(old)
			if !errors.Is(err, importer.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if !strings.Contains(err.Error(), "target") {
				t.Errorf("エラーに原因(target)が無い: %v", err)
			}
		})
	}
}

// TestOutputVersionReflectsMoveTarget は対象だけが変わっても変換結果の版(ADR-0122)が変わること。
// migration 000010 の直後は既存の行の target が NULL なので、取得元の版が同じでも次の取り込みで
// 必ず投入し直される必要がある(ADR-0136 §1)。
func TestOutputVersionReflectsMoveTarget(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	before := mustOutputVersion(t, out)
	changed := out
	changed.Moves = append([]importer.MoveRow(nil), out.Moves...)
	for i := range changed.Moves {
		if changed.Moves[i].ID == "teststrike" {
			changed.Moves[i].Target = "self"
		}
	}
	if after := mustOutputVersion(t, changed); after.Checksum == before.Checksum {
		t.Fatal("対象だけを変えても変換結果の版が変わらない(移行後の再投入が起きない)")
	}
}
