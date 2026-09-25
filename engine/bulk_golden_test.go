//go:build golden

package engine

// 一括計算(P1-7)のプリセット定義を、外部実装(@smogon/calc)と照合済みの
// 防御側網羅ベクタ testdata/golden/defense-species.jsonl.gz と突き合わせる。
//
// ベクタは tools/golden/generate.mjs が、ADR-0009 §6 の8件
//   none     = SP なし          / Serious(無補正)
//   hp       = hp:32            / Serious
//   hb_boost = hp:32            / Bold
//   hb       = hp:32, def:32    / Serious
//   hb_full  = hp:32, def:32    / Bold
//   hd_boost = hp:32            / Calm
//   hd       = hp:32, spd:32    / Serious
//   hd_full  = hp:32, spd:32    / Calm
// で生成する。engine の既定カタログ(ADR-0009 §1)がこれと1文字でも違えば、
// 一括計算の行はゴールデンと一致しない。ここで検出する。
//
// 件数・期待値を変えた理由: P1-10 の防御プリセット再定義(ユーザー決定)。
// 旧ベクタは zero/h/hb/hd の4件で、旧 hb(Bold 込み)は新 hb_full、旧 hd(Calm 込み)は新 hd_full に相当する。
// 改訂後のカタログは8件すべてが「SP と性格だけ」で定義されるため、全件を外部照合できる。

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// golden のベクタ名 → engine のプリセットキー。
// ADR-0009 §6: ベクタ名は PresetKey と同一文字列にする(対応表は存在検査で済む)。
var goldenDefensePresetKeys = map[string]PresetKey{
	"none":     PresetNone,
	"hp":       PresetHP,
	"hb_boost": PresetHBBoost,
	"hb":       PresetHB,
	"hb_full":  PresetHBFull,
	"hd_boost": PresetHDBoost,
	"hd":       PresetHD,
	"hd_full":  PresetHDFull,
}

// goldenDefensePresetsPerGroup は (種族 × 攻撃側アンカー) 1グループあたりのベクタ件数。
const goldenDefensePresetsPerGroup = 8

// goldenDefenseAnchors は防御側網羅ベクタの攻撃側アンカー数(generate.mjs の attackAnchors)。
const goldenDefenseAnchors = 5

// 外部照合(@smogon/calc)されているのがどのプリセットかを固定する。
// ADR-0009 §6: 改訂後は既定カタログの全キーが golden のベクタ名として存在しなければならない。
// カタログにプリセットを足したのに golden を足し忘れる、という退行をここで止める。
func TestGoldenCoversEveryDefenderPreset(t *testing.T) {
	catalog := DefenderPresetCatalog()
	if len(catalog) != goldenDefensePresetsPerGroup {
		t.Fatalf("カタログ件数=%d、golden の1グループ件数=%d。両方を同時に更新すること(ADR-0009 §6)",
			len(catalog), goldenDefensePresetsPerGroup)
	}
	seen := map[PresetKey]string{}
	for name, key := range goldenDefensePresetKeys {
		if prev, dup := seen[key]; dup {
			t.Errorf("プリセット %q に golden のベクタ名が2つ対応している: %q と %q", key, prev, name)
		}
		seen[key] = name
	}
	for _, p := range catalog {
		name, ok := seen[p.Key]
		if !ok {
			t.Errorf("カタログの %q に対応する golden ベクタが無い(外部照合されていない)", p.Key)
			continue
		}
		if name != string(p.Key) {
			t.Errorf("golden ベクタ名 %q はプリセットキー %q と同じ文字列であること(ADR-0009 §6)", name, p.Key)
		}
	}
	for key, name := range seen {
		found := false
		for _, p := range catalog {
			if p.Key == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("golden ベクタ %q に対応するプリセット %q がカタログに無い", name, key)
		}
	}
}

func TestGoldenBulkDefenderPresets(t *testing.T) {
	meta := readGoldenMetadata(t)
	const file = "defense-species.jsonl.gz"
	wantCount := meta.SpeciesCount * goldenDefenseAnchors * goldenDefensePresetsPerGroup
	if meta.Files[file].Count != wantCount {
		t.Fatalf("defense-species coverage incomplete: count=%d want %d(種族 %d × アンカー %d × プリセット %d)",
			meta.Files[file].Count, wantCount, meta.SpeciesCount, goldenDefenseAnchors, goldenDefensePresetsPerGroup)
	}

	var (
		group    []goldenCase
		groupKey string
		groups   int
		rows     int
		failures int
	)

	// ベクタは (種族, 攻撃側アンカー) ごとにカタログ順の8件が連続して並ぶ。
	// その8件を1回の CalcBulk で再現し、全行を照合する。
	flush := func() {
		if len(group) == 0 {
			return
		}
		groups++
		if len(group) != goldenDefensePresetsPerGroup {
			t.Fatalf("%s: グループの件数=%d want %d(ベクタの並び順の前提が崩れた)", group[0].ID, len(group), goldenDefensePresetsPerGroup)
		}
		first := group[0]
		in := BulkInput{
			Format:          first.Input.Format,
			Attacker:        first.Input.Attacker,
			DefenderSpecies: first.Input.Defender.Species,
			Move:            first.Input.Move,
			Field:           first.Input.Field,
			Critical:        first.Input.Critical,
		}
		for _, c := range group {
			name := c.ID[strings.LastIndex(c.ID, "/")+1:]
			key, ok := goldenDefensePresetKeys[name]
			if !ok {
				t.Fatalf("%s: 未知の golden プリセット名 %q", c.ID, name)
			}
			if !reflect.DeepEqual(c.Input.Attacker, first.Input.Attacker) ||
				!reflect.DeepEqual(c.Input.Move, first.Input.Move) || c.Input.Field != first.Input.Field ||
				c.Input.Critical != first.Input.Critical || c.Input.Format != first.Input.Format {
				t.Fatalf("%s: 同一グループで攻撃側/技/場が異なる(ベクタの並び順の前提が崩れた)", c.ID)
			}
			in.PresetKeys = append(in.PresetKeys, key)
		}

		res, err := calcBulk(in)
		if err != nil || len(res.Rows) != len(group) {
			failures += len(group)
			if failures <= 12 {
				t.Logf("%s: CalcBulk error=%v rows=%d want=%d", first.ID, err, len(res.Rows), len(group))
			}
			group = group[:0]
			return
		}
		for i, c := range group {
			rows++
			row := res.Rows[i]
			ko := c.Expected.KO
			d := c.Expected.DefenderStats
			mismatch := row.Preset != in.PresetKeys[i] ||
				!reflect.DeepEqual(row.Defender, c.Input.Defender) ||
				row.Result.Rolls != c.Expected.Rolls ||
				row.Result.DefenderHP != d.HP ||
				row.Result.KO.Hits != ko.Hits || row.Result.KO.Guaranteed != ko.Guaranteed ||
				math.Abs(row.Result.KO.ChancePercent-ko.ChancePercent) > 1e-9
			if mismatch {
				failures++
				if failures <= 12 {
					t.Logf("%s: preset=%q want=%q rolls=%v want=%v defender=%+v want=%+v KO=%+v want=%+v",
						c.ID, row.Preset, in.PresetKeys[i], row.Result.Rolls, c.Expected.Rolls,
						row.Defender, c.Input.Defender, row.Result.KO, ko)
				}
			}
		}
		group = group[:0]
	}

	eachGoldenLine(t, meta, file, func(line []byte) {
		var v goldenCase
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatal(err)
		}
		parts := strings.Split(v.ID, "/")
		if len(parts) != 4 || parts[0] != "defense" {
			t.Fatalf("想定外のベクタ ID: %q", v.ID)
		}
		key := parts[1] + "/" + parts[2]
		if key != groupKey {
			flush()
			groupKey = key
		}
		group = append(group, v)
	})
	flush()

	if groups != meta.SpeciesCount*goldenDefenseAnchors {
		t.Errorf("グループ数=%d want %d", groups, meta.SpeciesCount*goldenDefenseAnchors)
	}
	if failures == 0 && rows != meta.Files[file].Count {
		t.Errorf("照合した行数=%d want %d", rows, meta.Files[file].Count)
	}
	if failures > 0 {
		t.Errorf("%d/%d 行が外部照合済みベクタと不一致(ADR-0009 のプリセット定義と tools/golden の定義を突き合わせること)", failures, meta.Files[file].Count)
	}
}
