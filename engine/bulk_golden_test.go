//go:build golden

package engine

// 一括計算(P1-7)のプリセット定義を、外部実装(@smogon/calc)と照合済みの
// 防御側網羅ベクタ testdata/golden/defense-species.jsonl.gz と突き合わせる。
//
// ベクタは tools/golden/generate.mjs が
//   zero = SP なし / 無補正、h = hp:32、hb = hp:32,def:32 + Bold、hd = hp:32,spd:32 + Calm
// で生成している。engine の既定カタログ(ADR-0009)がこれと1文字でも違えば、
// 一括計算の行はゴールデンと一致しない。ここで検出する。

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// golden のプリセット名 → engine のプリセットキー。
var goldenDefensePresetKeys = map[string]PresetKey{
	"zero": PresetNone,
	"h":    PresetHP,
	"hb":   PresetHB,
	"hd":   PresetHD,
}

func TestGoldenBulkDefenderPresets(t *testing.T) {
	meta := readGoldenMetadata(t)
	const file = "defense-species.jsonl.gz"
	if meta.Files[file].Count != meta.SpeciesCount*20 {
		t.Fatal("defense-species coverage incomplete")
	}

	var (
		group    []goldenCase
		groupKey string
		groups   int
		rows     int
		failures int
	)

	// ベクタは (種族, 攻撃側アンカー) ごとに zero/h/hb/hd の4件が連続して並ぶ。
	// その4件を1回の CalcBulk で再現し、全行を照合する。
	flush := func() {
		if len(group) == 0 {
			return
		}
		groups++
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
				c.Input.Move != first.Input.Move || c.Input.Field != first.Input.Field ||
				c.Input.Critical != first.Input.Critical || c.Input.Format != first.Input.Format {
				t.Fatalf("%s: 同一グループで攻撃側/技/場が異なる(ベクタの並び順の前提が崩れた)", c.ID)
			}
			in.PresetKeys = append(in.PresetKeys, key)
		}

		res, err := CalcBulk(in)
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

	if groups != meta.SpeciesCount*5 {
		t.Errorf("グループ数=%d want %d", groups, meta.SpeciesCount*5)
	}
	if failures == 0 && rows != meta.Files[file].Count {
		t.Errorf("照合した行数=%d want %d", rows, meta.Files[file].Count)
	}
	if failures > 0 {
		t.Errorf("%d/%d 行が外部照合済みベクタと不一致(ADR-0009 のプリセット定義と tools/golden の定義を突き合わせること)", failures, meta.Files[file].Count)
	}
}
