package wasmapi_test

// P1-9 受け入れ条件 AC-2「境界は engine の素通しである」の検証。
//
// testdata/vectors.json の各ベクタについて、
//   (a) wasmapi.Calc / CalcBulk / CalcReverse の返す JSON
//   (b) 同じリクエストを engine の公開 API(CalcDamage / CalcBulk / CalcReverse)へ直接渡した結果
// の数値がすべて一致することを確かめる。DTO が独自計算・独自丸めを持ち込んでいないことの固定。
//
// 同じベクタは Node 側の一致テスト(scripts/wasm-conformance.mjs、`make test-wasm`)でも使う。
// ネイティブ側の期待値はコミットせず、実行のたびに engine/cmd/wasmexpect が生成する(ADR-0011 §7)。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/engine/wasmapi"
)

const vectorsPath = "testdata/vectors.json"

// vectorsSchemaVersion はベクタファイルの版。
// 2 で「先頭の typeChart を各リクエストに注入する」形になった(ADR-0011 §13)。
// 注入を実装していない読み手は、ここで必ず落ちる(黙って素通りさせない)。
const vectorsSchemaVersion = 2

// typeChartTypeCount は相性表のタイプ数(第9世代 / チャンピオンズ)。
const typeChartTypeCount = 18

type vector struct {
	Name    string          `json:"name"`
	Fn      string          `json:"fn"`
	Tags    []string        `json:"tags"`
	Source  string          `json:"source"`
	Request json.RawMessage `json:"request"`
}

// typeChartDoc はベクタ先頭で1度だけ定義する相性表(ADR-0013 §P1-13.4)。
type typeChartDoc struct {
	Types         []engine.Type                       `json:"types"`
	Effectiveness map[engine.Type]map[engine.Type]int `json:"effectiveness"`
}

type vectorFile struct {
	SchemaVersion int             `json:"schemaVersion"`
	TypeChart     typeChartDoc    `json:"typeChart"`
	TypeChartRaw  json.RawMessage `json:"-"`
	Vectors       []vector        `json:"vectors"`
}

// loadVectorFile はベクタと共有の相性表を1度だけ読む。
var loadVectorFile = sync.OnceValues(func() (vectorFile, error) {
	var f vectorFile
	b, err := os.ReadFile(vectorsPath)
	if err != nil {
		return f, fmt.Errorf("ベクタを読めない: %w", err)
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, fmt.Errorf("ベクタの JSON が壊れている: %w", err)
	}
	if f.SchemaVersion != vectorsSchemaVersion {
		return f, fmt.Errorf("ベクタの schemaVersion=%d は未知(want %d)", f.SchemaVersion, vectorsSchemaVersion)
	}
	if len(f.Vectors) == 0 {
		return f, fmt.Errorf("ベクタが空")
	}
	if len(f.TypeChart.Types) == 0 {
		return f, fmt.Errorf("ベクタ先頭の typeChart が無い(ADR-0011 §13)")
	}
	// リクエストへ注入する生の JSON も取っておく(DTO を経由せずそのまま載せる)。
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return f, err
	}
	f.TypeChartRaw = top["typeChart"]
	return f, nil
})

func loadVectors(t *testing.T) []vector {
	t.Helper()
	f, err := loadVectorFile()
	if err != nil {
		t.Fatalf("%v", err)
	}
	return f.Vectors
}

// sharedTypeChart はベクタ先頭の表を engine の値にしたもの。
func sharedTypeChart(t *testing.T) engine.TypeChart {
	t.Helper()
	f, err := loadVectorFile()
	if err != nil {
		t.Fatalf("%v", err)
	}
	c, err := engine.NewTypeChart(engine.TypeChartData{Types: f.TypeChart.Types, Effectiveness: f.TypeChart.Effectiveness})
	if err != nil {
		t.Fatalf("ベクタの typeChart から表を作れない: %v", err)
	}
	return c
}

// requestWithTypeChart はリクエストに共有の typeChart を注入する(ADR-0011 §13)。
// 比較対象はレスポンスなので、キーの順序が変わっても一致テストには影響しない。
func requestWithTypeChart(t *testing.T, req json.RawMessage) string {
	t.Helper()
	f, err := loadVectorFile()
	if err != nil {
		t.Fatalf("%v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(req, &m); err != nil {
		t.Fatalf("ベクタの request が object ではない: %v", err)
	}
	if _, dup := m["typeChart"]; dup {
		t.Fatal("リクエストに typeChart が直書きされている(表はファイル先頭で1度だけ定義する)")
	}
	m["typeChart"] = f.TypeChartRaw
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("リクエストを組み立てられない: %v", err)
	}
	return string(b)
}

// TestVectorsDefineSharedTypeChart はベクタが表を1度だけ定義していることを確かめる
// (各リクエストに 18×18 を書かない。ADR-0011 §13)。
func TestVectorsDefineSharedTypeChart(t *testing.T) {
	f, err := loadVectorFile()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(f.TypeChart.Types) != typeChartTypeCount {
		t.Errorf("typeChart のタイプ数 = %d, want %d", len(f.TypeChart.Types), typeChartTypeCount)
	}
	// 表の正しさは make test-golden の担当。ここでは代表的なマッチアップだけ固定する。
	chart := sharedTypeChart(t)
	for _, tt := range []struct {
		atk, def engine.Type
		want     int
	}{
		{engine.TypeFire, engine.TypeGrass, 4},
		{engine.TypeNormal, engine.TypeGhost, 0},
		{engine.TypeWater, engine.TypeFire, 4},
		{engine.TypeNormal, engine.TypePsychic, 2}, // 等倍は省略されている
	} {
		got, err := chart.Code(tt.atk, tt.def)
		if err != nil {
			t.Fatalf("Code(%s, %s): %v", tt.atk, tt.def, err)
		}
		if got != tt.want {
			t.Errorf("Code(%s, %s) = %d, want %d", tt.atk, tt.def, got, tt.want)
		}
	}
	// どのベクタも表を直書きしていないこと。
	for _, v := range f.Vectors {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(v.Request, &m); err != nil {
			t.Fatalf("%s: request が object ではない: %v", v.Name, err)
		}
		if _, dup := m["typeChart"]; dup {
			t.Errorf("%s が typeChart を直書きしている", v.Name)
		}
	}
}

func vectorsFor(t *testing.T, fn string) []vector {
	t.Helper()
	var out []vector
	for _, v := range loadVectors(t) {
		if v.Fn == fn {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		t.Fatalf("fn=%q のベクタが無い", fn)
	}
	return out
}

// --- レスポンスの型(= 境界の契約。ADR-0011 §4)------------------------------

type statsView struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

func statsOf(s engine.Stats) statsView {
	return statsView{HP: s.HP, Atk: s.Atk, Def: s.Def, SpA: s.SpA, SpD: s.SpD, Spe: s.Spe}
}

type natureView struct {
	Plus  string `json:"plus"`
	Minus string `json:"minus"`
}

func natureOf(n engine.Nature) natureView {
	return natureView{Plus: string(n.Plus), Minus: string(n.Minus)}
}

// tenths は表示%(0.1% 単位。ADR-0010 §3.2)のテキスト "73.4" を 734 に戻す。
// JSON の生テキストのまま受けるので、「値」だけでなく「書式」も見られる。
// 小数第1位がちょうど1桁でなければ失敗させる(ADR-0011 §3)。
func tenths(t *testing.T, what string, v json.Number) int {
	t.Helper()
	s := v.String()
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart, fracPart, ok := strings.Cut(s, ".")
	if !ok || len(fracPart) != 1 || intPart == "" {
		t.Fatalf("%s は小数第1位を1桁だけ持つべき: %q", what, v.String())
	}
	n, err := strconv.Atoi(intPart + fracPart)
	if err != nil {
		t.Fatalf("%s が数値でない: %q", what, v.String())
	}
	if neg {
		n = -n
	}
	return n
}

type koView struct {
	Hits                 int         `json:"hits"`
	Guaranteed           bool        `json:"guaranteed"`
	ChancePercent        float64     `json:"chancePercent"`
	DisplayChancePercent json.Number `json:"displayChancePercent"`
}

type calcResultView struct {
	Rolls         [16]int     `json:"rolls"`
	MinDamage     int         `json:"minDamage"`
	MaxDamage     int         `json:"maxDamage"`
	MinPercent    json.Number `json:"minPercent"`
	MaxPercent    json.Number `json:"maxPercent"`
	DefenderHP    int         `json:"defenderHP"`
	Effectiveness float64     `json:"effectiveness"`
	STAB          bool        `json:"stab"`
	Category      string      `json:"category"`
	KO            koView      `json:"ko"`
}

type bulkDefenderView struct {
	SP     statsView  `json:"sp"`
	Nature natureView `json:"nature"`
	Stats  statsView  `json:"stats"`
}

type bulkRowView struct {
	Preset      string           `json:"preset"`
	PresetLabel string           `json:"presetLabel"`
	ItemID      string           `json:"itemId"`
	Defender    bulkDefenderView `json:"defender"`
	Result      calcResultView   `json:"result"`
}

type bulkResultView struct {
	DefenderSpeciesKey string        `json:"defenderSpeciesKey"`
	Rows               []bulkRowView `json:"rows"`
}

type archetypeView struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Side        string `json:"side"`
	Stat        string `json:"stat"`
	HPBucket    string `json:"hpBucket"`
	StatBucket  string `json:"statBucket"`
	NatureClass string `json:"natureClass"`
}

type reverseCandidateView struct {
	Archetype   archetypeView `json:"archetype"`
	ItemID      string        `json:"itemId"`
	SP          statsView     `json:"sp"`
	Nature      natureView    `json:"nature"`
	MatchScore  float64       `json:"matchScore"`
	Exact       bool          `json:"exact"`
	MinPercent  json.Number   `json:"minPercent"`
	MaxPercent  json.Number   `json:"maxPercent"`
	Points      int           `json:"points"`
	ExactPoints int           `json:"exactPoints"`
}

type reverseResultView struct {
	Side       string                 `json:"side"`
	Stat       string                 `json:"stat"`
	ExactCount int                    `json:"exactCount"`
	Candidates []reverseCandidateView `json:"candidates"`
}

type errorView struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// decodeEnvelope は封筒を割り、result を out へ入れる。error 封筒なら t.Fatal。
func decodeEnvelope(t *testing.T, resp string, out any) {
	t.Helper()
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *errorView      `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp), &env); err != nil {
		t.Fatalf("レスポンスが JSON ではない: %v\n%s", err, resp)
	}
	if env.Error != nil {
		t.Fatalf("成功するはずが error 封筒: code=%q message=%q", env.Error.Code, env.Error.Message)
	}
	if len(env.Result) == 0 {
		t.Fatalf("result が無い: %s", resp)
	}
	dec := json.NewDecoder(bytes.NewReader(env.Result))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		t.Fatalf("result が契約と合わない(未知フィールド/型違い): %v\n%s", err, env.Result)
	}
}

// --- AC-2: engine の素通しであること ---------------------------------------

func TestCalcMatchesEngineCalcDamage(t *testing.T) {
	chart := sharedTypeChart(t)
	for _, v := range vectorsFor(t, "calc") {
		t.Run(v.Name, func(t *testing.T) {
			var in engine.DamageInput
			if err := json.Unmarshal(v.Request, &in); err != nil {
				t.Fatalf("ベクタを engine.DamageInput にできない: %v", err)
			}
			in.TypeChart = chart // 表はベクタ先頭の1つを注入する(ADR-0011 §13)
			want, err := engine.CalcDamage(in)
			if err != nil {
				t.Fatalf("engine.CalcDamage が失敗(ベクタが不正): %v", err)
			}

			var got calcResultView
			decodeEnvelope(t, wasmapi.Calc(requestWithTypeChart(t, v.Request)), &got)
			assertCalcResult(t, got, want)
		})
	}
}

func assertCalcResult(t *testing.T, got calcResultView, want engine.DamageResult) {
	t.Helper()
	if got.Rolls != want.Rolls {
		t.Errorf("rolls が engine と違う\n got %v\nwant %v", got.Rolls, want.Rolls)
	}
	if got.MinDamage != want.MinDamage() || got.MaxDamage != want.MaxDamage() {
		t.Errorf("minDamage/maxDamage: got %d/%d want %d/%d", got.MinDamage, got.MaxDamage, want.MinDamage(), want.MaxDamage())
	}
	// 表示%(0.1% 単位)は engine の値の素通し。最小側=切り捨て、最大側=四捨五入(ADR-0010 §3.2)。
	wantMin, wantMax := want.DisplayPercentRangeTenths()
	gotMin := tenths(t, "minPercent", got.MinPercent)
	gotMax := tenths(t, "maxPercent", got.MaxPercent)
	if gotMin != wantMin || gotMax != wantMax {
		t.Errorf("minPercent/maxPercent: got %d/%d want %d/%d(0.1%%単位。DamageResult.DisplayPercentRangeTenths)",
			gotMin, gotMax, wantMin, wantMax)
	}
	if got.DefenderHP != want.DefenderHP {
		t.Errorf("defenderHP: got %d want %d", got.DefenderHP, want.DefenderHP)
	}
	if got.Effectiveness != want.Effectiveness {
		t.Errorf("effectiveness: got %v want %v", got.Effectiveness, want.Effectiveness)
	}
	if got.STAB != want.STAB {
		t.Errorf("stab: got %v want %v", got.STAB, want.STAB)
	}
	if got.Category != string(want.Category) {
		t.Errorf("category: got %q want %q", got.Category, want.Category)
	}
	if got.KO.Hits != want.KO.Hits || got.KO.Guaranteed != want.KO.Guaranteed || got.KO.ChancePercent != want.KO.ChancePercent {
		t.Errorf("ko: got %+v want %+v", got.KO, want.KO)
	}
	// 表示用の確率も engine の素通し(確定なら 100.0%、倒せないなら 0.0%。ADR-0010 §3.4)。
	if gotChance, wantChance := tenths(t, "ko.displayChancePercent", got.KO.DisplayChancePercent),
		want.KO.DisplayChancePercentTenths(); gotChance != wantChance {
		t.Errorf("ko.displayChancePercent: got %d want %d(0.1%%単位)", gotChance, wantChance)
	}
}

func TestCalcBulkMatchesEngineCalcBulk(t *testing.T) {
	chart := sharedTypeChart(t)
	for _, v := range vectorsFor(t, "calcBulk") {
		t.Run(v.Name, func(t *testing.T) {
			var in engine.BulkInput
			if err := json.Unmarshal(v.Request, &in); err != nil {
				t.Fatalf("ベクタを engine.BulkInput にできない: %v", err)
			}
			in.TypeChart = chart
			want, err := engine.CalcBulk(in)
			if err != nil {
				t.Fatalf("engine.CalcBulk が失敗(ベクタが不正): %v", err)
			}

			var got bulkResultView
			decodeEnvelope(t, wasmapi.CalcBulk(requestWithTypeChart(t, v.Request)), &got)

			if got.DefenderSpeciesKey != want.DefenderSpeciesKey {
				t.Errorf("defenderSpeciesKey: got %q want %q", got.DefenderSpeciesKey, want.DefenderSpeciesKey)
			}
			if len(got.Rows) != len(want.Rows) {
				t.Fatalf("行数: got %d want %d(順序も engine と同じでなければならない)", len(got.Rows), len(want.Rows))
			}
			for i, wr := range want.Rows {
				gr := got.Rows[i]
				if gr.Preset != string(wr.Preset) || gr.PresetLabel != wr.PresetLabel || gr.ItemID != wr.ItemID {
					t.Errorf("行 %d のキー: got %q/%q/%q want %q/%q/%q", i, gr.Preset, gr.PresetLabel, gr.ItemID, wr.Preset, wr.PresetLabel, wr.ItemID)
				}
				if gr.Defender.SP != statsOf(wr.Defender.SP) {
					t.Errorf("行 %d の defender.sp: got %+v want %+v", i, gr.Defender.SP, statsOf(wr.Defender.SP))
				}
				if gr.Defender.Nature != natureOf(wr.Defender.Nature) {
					t.Errorf("行 %d の defender.nature: got %+v want %+v", i, gr.Defender.Nature, natureOf(wr.Defender.Nature))
				}
				if want := statsOf(engine.RealStats(wr.Defender)); gr.Defender.Stats != want {
					t.Errorf("行 %d の defender.stats: got %+v want %+v(engine.RealStats)", i, gr.Defender.Stats, want)
				}
				assertCalcResult(t, gr.Result, wr.Result)
			}
		})
	}
}

func TestCalcReverseMatchesEngineCalcReverse(t *testing.T) {
	chart := sharedTypeChart(t)
	for _, v := range vectorsFor(t, "calcReverse") {
		t.Run(v.Name, func(t *testing.T) {
			var in engine.ReverseInput
			if err := json.Unmarshal(v.Request, &in); err != nil {
				t.Fatalf("ベクタを engine.ReverseInput にできない: %v", err)
			}
			in.TypeChart = chart
			want, err := engine.CalcReverse(in)
			if err != nil {
				t.Fatalf("engine.CalcReverse が失敗(ベクタが不正): %v", err)
			}

			var got reverseResultView
			decodeEnvelope(t, wasmapi.CalcReverse(requestWithTypeChart(t, v.Request)), &got)

			if got.Side != string(want.Side) || got.Stat != string(want.Stat) || got.ExactCount != want.ExactCount {
				t.Errorf("side/stat/exactCount: got %q/%q/%d want %q/%q/%d", got.Side, got.Stat, got.ExactCount, want.Side, want.Stat, want.ExactCount)
			}
			if len(got.Candidates) != len(want.Candidates) {
				t.Fatalf("候補数: got %d want %d(順序も engine と同じでなければならない)", len(got.Candidates), len(want.Candidates))
			}
			for i, wc := range want.Candidates {
				gc := got.Candidates[i]
				wantArch := archetypeView{
					Key: string(wc.Archetype.Key), Label: wc.Archetype.Label,
					Side: string(wc.Archetype.Side), Stat: string(wc.Archetype.Stat),
					HPBucket: string(wc.Archetype.HPBucket), StatBucket: string(wc.Archetype.StatBucket),
					NatureClass: string(wc.Archetype.NatureClass),
				}
				if gc.Archetype != wantArch {
					t.Errorf("候補 %d の archetype: got %+v want %+v", i, gc.Archetype, wantArch)
				}
				if gc.ItemID != wc.ItemID {
					t.Errorf("候補 %d の itemId: got %q want %q", i, gc.ItemID, wc.ItemID)
				}
				if gc.SP != statsOf(wc.SP) || gc.Nature != natureOf(wc.Nature) {
					t.Errorf("候補 %d の sp/nature: got %+v/%+v want %+v/%+v", i, gc.SP, gc.Nature, statsOf(wc.SP), natureOf(wc.Nature))
				}
				if gc.MatchScore != wc.MatchScore || gc.Exact != wc.Exact {
					t.Errorf("候補 %d の matchScore/exact: got %v/%v want %v/%v", i, gc.MatchScore, gc.Exact, wc.MatchScore, wc.Exact)
				}
				// 候補の想定ダメージ幅も表示%(0.1% 単位)。CalcResult と同じ意味・同じ書式
				// でなければならない(ADR-0010 §3.3)。
				gcMin := tenths(t, "候補の minPercent", gc.MinPercent)
				gcMax := tenths(t, "候補の maxPercent", gc.MaxPercent)
				if gcMin != wc.MinPercentTenths || gcMax != wc.MaxPercentTenths {
					t.Errorf("候補 %d の minPercent/maxPercent: got %d/%d want %d/%d(0.1%%単位)",
						i, gcMin, gcMax, wc.MinPercentTenths, wc.MaxPercentTenths)
				}
				if gc.Points != wc.Points || gc.ExactPoints != wc.ExactPoints {
					t.Errorf("候補 %d の points/exactPoints: got %d/%d want %d/%d", i, gc.Points, gc.ExactPoints, wc.Points, wc.ExactPoints)
				}
			}
		})
	}
}

// --- AC-8: ベクタが必要な条件を覆っていること --------------------------------

func TestVectorsCoverRequiredScenarios(t *testing.T) {
	vs := loadVectors(t)

	tags := map[string]int{}
	fns := map[string]int{}
	names := map[string]bool{}
	for _, v := range vs {
		if names[v.Name] {
			t.Errorf("ベクタ名が重複している: %q", v.Name)
		}
		names[v.Name] = true
		fns[v.Fn]++
		for _, tag := range v.Tags {
			tags[tag]++
		}
	}

	for _, fn := range []string{"calc", "calcBulk", "calcReverse"} {
		if fns[fn] == 0 {
			t.Errorf("fn=%q のベクタが無い", fn)
		}
	}
	// 「ダメージ(補正を通るもの)・一括・逆算」の3系統(P1-9 のスコープ)。
	required := []string{
		"weather", "screens", "item", "ability", "critical", "burn", "terrain", "ranks", "effectiveness",
		"bulk", "itemVariants",
		"reverse", "percent", "damage", "defender", "attacker", "multiObservation", "itemCandidates",
		"float", "defaults",
	}
	for _, tag := range required {
		if tags[tag] == 0 {
			t.Errorf("必須タグ %q のベクタが無い", tag)
		}
	}
	if fns["calc"] < 15 {
		t.Errorf("calc ベクタが %d 件しかない(補正を1つずつ通すため 15 件以上)", fns["calc"])
	}
}
