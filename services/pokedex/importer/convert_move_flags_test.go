package importer_test

// 技のフラグ(move_flags。ADR-0178)の取り込みと照合。
//
// 受け入れ条件(importer):
//   - AC-I1 変換: moves 表に採る技(変化技を含む)ごとに、Showdown の値からフラグを導いて Output.MoveFlags に入れる。
//     contact・sound・punch・bite・slicing・pulse・bullet は flags の同名のキー、recoil は recoil があるか hasCrashDamage、
//     secondary は secondary があるか secondaries が空でない。語彙に無い Showdown のフラグ(protect 等)は捨てる。
//     行は (技 ID, フラグ) の昇順・重複なし。フラグの無い技は行を作らない。moves 表に採らない技は行を作らない。
//   - AC-I2 照合: calc と Showdown の両方にある技で、calc から同じ規則で導いたフラグの集合が Showdown と違えば
//     move-value-mismatch(Detail "flags")。攻撃技は Blocker、変化技は警告。値は Showdown を採る。
//   - AC-I3 取得物: Showdown・calc とも flags キーが無い古い取得物はデコードで ErrInvalidInput(メッセージに flags)。
//     Input を直接組んで Flags が nil なら ErrInvalidData(黙ってフラグなしにしない)。
//   - AC-I4 変換結果の版(ADR-0122)はフラグだけが変わっても変わる(migrate 後の最初の取り込みで必ず入れ直す)。
//   - AC-I5 照合の要約に moveFlags: attack=<攻撃技数> withFlag=<フラグを持つ攻撃技数> とフラグごとの件数を出す。
//
// 架空データ(testdata/fictional)が持つ形:
//
//	testflame   Showdown flags [metronome mirror protect sound]・secondary あり → secondary・sound(calc: sound・secondaries)
//	teststrike  Showdown flags [contact mirror protect punch]・recoil [33,100] → contact・punch・recoil(calc も同じ)
//	testglare   変化技。Showdown flags [reflectable sound] → sound(calc も sound)
//	testrevived calc は断片。Showdown flags [bullet protect]・secondary あり → bullet・secondary
//	testsplash  Showdown だけ。flags [protect pulse]・hasCrashDamage・secondary あり → pulse・recoil・secondary
//	testold・testbanned は moves 表に採らない技(行を作らない)

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func moveFlagsByID(out importer.Output) map[string][]string {
	m := map[string][]string{}
	for _, r := range out.MoveFlags {
		m[r.MoveID] = append(m[r.MoveID], r.Flag)
	}
	return m
}

func flagList(fs ...string) *[]string {
	out := append([]string{}, fs...)
	return &out
}

func hasFlagsFinding(findings []importer.Finding, id string) bool {
	for _, f := range findings {
		if f.Kind == importer.KindMoveValueMismatch && f.ID == id && f.Detail == "flags" {
			return true
		}
	}
	return false
}

// AC-I1
func TestConvertMoveFlagsFromFixture(t *testing.T) {
	out, rep := convertOK(t, loadFixture(t))
	want := map[string][]string{
		"testflame":   {"secondary", "sound"},
		"teststrike":  {"contact", "punch", "recoil"},
		"testglare":   {"sound"},
		"testrevived": {"bullet", "secondary"},
		"testsplash":  {"pulse", "recoil", "secondary"},
	}
	if got := moveFlagsByID(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("move_flags = %v,\nwant %v", got, want)
	}
	for _, f := range append(append([]importer.Finding{}, rep.Warnings...), rep.Blockers...) {
		if f.Kind == importer.KindMoveValueMismatch && f.Detail == "flags" {
			t.Errorf("fixture は calc と Showdown のフラグが一致しているのに食い違いが出た: %+v", f)
		}
	}
}

func TestConvertMoveFlagsAreSortedAndUnique(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	rows := out.MoveFlags
	if len(rows) == 0 {
		t.Fatal("move_flags が空")
	}
	if !sort.SliceIsSorted(rows, func(i, j int) bool {
		if rows[i].MoveID != rows[j].MoveID {
			return rows[i].MoveID < rows[j].MoveID
		}
		return rows[i].Flag < rows[j].Flag
	}) {
		t.Fatalf("move_flags が昇順でない: %+v", rows)
	}
	seen := map[importer.MoveFlagRow]bool{}
	for _, r := range rows {
		if seen[r] {
			t.Fatalf("move_flags に重複: %+v", r)
		}
		seen[r] = true
	}
}

// Showdown の値の組み合わせごとの導き方(両方にある攻撃技 teststrike で確かめる。calc も同じ値に合わせて食い違いを出さない)。
func TestConvertMoveFlagsDerivation(t *testing.T) {
	cases := []struct {
		name        string
		flags       []string
		recoil      string
		crash       bool
		secondary   bool
		want        []string
		calcFlags   []string
		calcSec     bool
		calcRecoilN string
	}{
		{"語彙のキーだけを採る", []string{"bite", "bullet", "contact", "mirror", "pulse", "punch", "slicing", "sound", "wind"}, "null", false, false,
			[]string{"bite", "bullet", "contact", "pulse", "punch", "slicing", "sound"}, []string{"bite", "bullet", "contact", "pulse", "punch", "slicing", "sound"}, false, "null"},
		{"フラグなし", []string{"protect"}, "null", false, false, nil, []string{}, false, "null"},
		{"反動(recoil)", []string{}, "[1, 3]", false, false, []string{"recoil"}, []string{}, false, "[1, 3]"},
		{"反動(hasCrashDamage)", []string{}, "null", true, false, []string{"recoil"}, []string{}, false, "null"},
		{"追加効果", []string{}, "null", false, true, []string{"secondary"}, []string{}, true, "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			sm := showdownMove(t, &in, "teststrike")
			sm.Flags = flagList(tc.flags...)
			sm.Recoil = json.RawMessage(tc.recoil)
			sm.HasCrashDamage = tc.crash
			sm.Secondary = nil
			sm.Secondaries = nil
			if tc.secondary {
				sm.Secondaries = []importer.ShowdownSecondary{{}}
			}
			cm := calcMove(t, &in, "Test Strike")
			cm.Flags = flagList(tc.calcFlags...)
			cm.Recoil = json.RawMessage(tc.calcRecoilN)
			cm.HasCrashDamage = tc.crash
			cm.Secondaries = tc.calcSec
			out, _ := convertOK(t, in)
			got := moveFlagsByID(out)["teststrike"]
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("teststrike のフラグ = %v, want %v", got, tc.want)
			}
		})
	}
}

// AC-I2
func TestConvertMoveFlagsMatchCalc(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(t *testing.T, in *importer.Input)
		id      string
		blocked bool
	}{
		{"calc にパンチが無い(攻撃技)", func(t *testing.T, in *importer.Input) {
			calcMove(t, in, "Test Strike").Flags = flagList("contact")
		}, "teststrike", true},
		{"calc に余分な弾がある(攻撃技)", func(t *testing.T, in *importer.Input) {
			calcMove(t, in, "Test Strike").Flags = flagList("bullet", "contact", "punch")
		}, "teststrike", true},
		{"calc に反動が無い(攻撃技)", func(t *testing.T, in *importer.Input) {
			calcMove(t, in, "Test Strike").Recoil = json.RawMessage("null")
		}, "teststrike", true},
		{"calc に追加効果が無い(攻撃技)", func(t *testing.T, in *importer.Input) {
			calcMove(t, in, "Test Flame").Secondaries = false
		}, "testflame", true},
		{"calc に音が無い(変化技)", func(t *testing.T, in *importer.Input) {
			calcMove(t, in, "Test Glare").Flags = flagList()
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
				if !hasFlagsFinding(rep.Blockers, tc.id) {
					t.Errorf("Blockers に move-value-mismatch/%s(flags)が無い: %+v", tc.id, rep.Blockers)
				}
				if !reflect.DeepEqual(out, importer.Output{}) {
					t.Error("止めたのに Output が空でない(部分的な結果を投入に回さない)")
				}
				return
			}
			if err != nil {
				t.Fatalf("Convert: %v(変化技のフラグの食い違いで止めない)", err)
			}
			if !hasFlagsFinding(rep.Warnings, tc.id) {
				t.Errorf("Warnings に move-value-mismatch/%s(flags)が無い: %+v", tc.id, rep.Warnings)
			}
			// 値は Showdown を採る。
			if got := moveFlagsByID(out)[tc.id]; !reflect.DeepEqual(got, []string{"sound"}) {
				t.Errorf("%s のフラグ = %v, want Showdown の値 [sound]", tc.id, got)
			}
		})
	}
}

// AC-I3
func TestConvertRejectsMissingMoveFlags(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, in *importer.Input)
	}{
		{"Showdown(両方にある技)", func(t *testing.T, in *importer.Input) { showdownMove(t, in, "teststrike").Flags = nil }},
		{"Showdown(Showdown だけの技)", func(t *testing.T, in *importer.Input) { showdownMove(t, in, "testsplash").Flags = nil }},
		{"calc", func(t *testing.T, in *importer.Input) { calcMove(t, in, "Test Strike").Flags = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			tc.mutate(t, &in)
			if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
				t.Fatalf("err = %v, want ErrInvalidData", err)
			}
		})
	}
}

func TestDecodeSnapshotsRequireMoveFlags(t *testing.T) {
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
			delete(moves[0].(map[string]any), "flags")
			old, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			err = tc.decode(old)
			if !errors.Is(err, importer.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput", err)
			}
			if !strings.Contains(err.Error(), "flags") {
				t.Errorf("エラーに原因(flags)が無い: %v", err)
			}
		})
	}
}

// AC-I4
func TestOutputVersionReflectsMoveFlags(t *testing.T) {
	out, _ := convertOK(t, loadFixture(t))
	if len(out.MoveFlags) == 0 {
		t.Fatal("move_flags が空(変換が行を作っていない)")
	}
	before := mustOutputVersion(t, out)
	changed := out
	changed.MoveFlags = append([]importer.MoveFlagRow(nil), out.MoveFlags[1:]...)
	if after := mustOutputVersion(t, changed); after.Checksum == before.Checksum {
		t.Fatal("フラグだけを変えても変換結果の版が変わらない(移行後の再投入が起きない)")
	}
}

// AC-I5
func TestReconcileSummaryCountsMoveFlags(t *testing.T) {
	_, rec := reconcileOK(t, reconcileInput(t))
	// moves 表の攻撃技: testflame・teststrike・testrevived・testsplash(testglare は変化技)。
	want := importer.MoveFlagSummary{
		Attack:   4,
		WithFlag: 4,
		ByFlag:   map[string]int{"bullet": 1, "contact": 1, "pulse": 1, "punch": 1, "recoil": 2, "secondary": 3, "sound": 1},
	}
	if !reflect.DeepEqual(rec.Summary.MoveFlags, want) {
		t.Errorf("Summary.MoveFlags = %+v, want %+v", rec.Summary.MoveFlags, want)
	}
	s := importer.FormatSummary(rec)
	for _, line := range []string{
		"moveFlags: attack=4 withFlag=4\n",
		"moveFlag recoil: 2\n",
		"moveFlag secondary: 3\n",
	} {
		if !strings.Contains(s, line) {
			t.Errorf("要約に %q が無い:\n%s", line, s)
		}
	}
}
