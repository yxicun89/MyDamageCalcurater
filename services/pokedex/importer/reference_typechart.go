package importer

// 参照の相性表との照合(issue #280・ADR-0118)。
//
// DB の types/type_chart は calc スナップショットから作る(convertTypes)。一方でゴールデンテスト・
// balance・Web は tools/golden が同じ版の calc から生成した testdata/golden/typechart.json を使う。
// 2つは同じ取得元の別経路の生成物なので、食い違ったまま取り込むと calc-svc と他の利用者が別の表で
// 動く。実データはテストに置けないため、import の照合で比べ、食い違いは Blocker にする。

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"example.com/pokecalc/services/internal/master"
)

// ReferenceTypeChart は testdata/golden/typechart.json の形(ADR-0013 §P1-13.5)。
// 倍率コードは 0=無効・1=いまひとつ・2=等倍・4=ばつぐん。全タイプの組を持つ(等倍も省かない)。
type ReferenceTypeChart struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Source        string                    `json:"source"`
	Version       string                    `json:"version"`
	Generation    int                       `json:"generation"`
	Note          string                    `json:"note"`
	ExcludedTypes []string                  `json:"excludedTypes"`
	Types         []string                  `json:"types"`
	Effectiveness map[string]map[string]int `json:"effectiveness"`
}

// neutralCode は等倍の倍率コード(importer の行は等倍の組を省く)。
const neutralCode = 2

var validTypeChartCodes = map[int]bool{0: true, 1: true, 2: true, 4: true}

// DecodeReferenceTypeChart は参照の相性表をデコードし、全タイプの組がそろっていることを確かめる。
func DecodeReferenceTypeChart(raw []byte) (ReferenceTypeChart, error) {
	var c ReferenceTypeChart
	if err := strictDecode(raw, &c); err != nil {
		return ReferenceTypeChart{}, err
	}
	if err := checkSchemaVersion(c.SchemaVersion); err != nil {
		return ReferenceTypeChart{}, err
	}
	if c.Version == "" {
		return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の version が空", ErrInvalidInput)
	}
	if len(c.Types) == 0 {
		return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の types が空", ErrInvalidInput)
	}
	typeSet := make(map[string]bool, len(c.Types))
	for _, id := range c.Types {
		if typeSet[id] {
			return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の types に重複: %q", ErrInvalidInput, id)
		}
		typeSet[id] = true
	}
	if len(c.Effectiveness) != len(c.Types) {
		return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の攻撃側の行が %d 件(types は %d 件)", ErrInvalidInput, len(c.Effectiveness), len(c.Types))
	}
	for atk, row := range c.Effectiveness {
		if !typeSet[atk] {
			return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の攻撃側に types に無いタイプ: %q", ErrInvalidInput, atk)
		}
		if len(row) != len(c.Types) {
			return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の攻撃側 %q の防御側が %d 件(types は %d 件)", ErrInvalidInput, atk, len(row), len(c.Types))
		}
		for def, code := range row {
			if !typeSet[def] {
				return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の防御側に types に無いタイプ: %q(攻撃側 %q)", ErrInvalidInput, def, atk)
			}
			if !validTypeChartCodes[code] {
				return ReferenceTypeChart{}, fmt.Errorf("%w: 参照の相性表の倍率コードが不正: %s>%s = %d", ErrInvalidInput, atk, def, code)
			}
		}
	}
	return c, nil
}

// LoadReferenceTypeChart は path の参照の相性表を読む。無い・読めないときは ErrInvalidInput
// (照合を黙って飛ばさない)。
func LoadReferenceTypeChart(path string) (*ReferenceTypeChart, error) {
	raw, err := readRequired(path)
	if err != nil {
		return nil, err
	}
	c, err := DecodeReferenceTypeChart(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

// CompareReferenceTypeChart は importer の変換結果(types と、等倍を省いた相性表の行)を参照の相性表と
// 比べ、食い違いを KindTypeChartReferenceMismatch の指摘として返す(純粋)。ID は
//   - "version": 参照の版が calcVersion(config.json の sources.calc)と違う
//   - "types": タイプ ID の集合が違う(Detail に片方だけのタイプ)
//   - "<攻撃>><防御>": 両方にあるタイプの組で倍率コードが違う(Detail に両方の値)
func CompareReferenceTypeChart(ref ReferenceTypeChart, calcVersion string, types []master.TypeRow, chart []master.TypeChartRow) []Finding {
	var findings []Finding
	if ref.Version != calcVersion {
		findings = append(findings, Finding{
			Kind:   KindTypeChartReferenceMismatch,
			ID:     "version",
			Detail: fmt.Sprintf("参照の相性表は %q、取り込む calc は %q", ref.Version, calcVersion),
		})
	}

	importedSet := make(map[string]bool, len(types))
	for _, r := range types {
		importedSet[r.ID] = true
	}
	refSet := make(map[string]bool, len(ref.Types))
	for _, id := range ref.Types {
		refSet[id] = true
	}
	var onlyImported, onlyRef []string
	for id := range importedSet {
		if !refSet[id] {
			onlyImported = append(onlyImported, id)
		}
	}
	for id := range refSet {
		if !importedSet[id] {
			onlyRef = append(onlyRef, id)
		}
	}
	if len(onlyImported) > 0 || len(onlyRef) > 0 {
		sort.Strings(onlyImported)
		sort.Strings(onlyRef)
		findings = append(findings, Finding{
			Kind:   KindTypeChartReferenceMismatch,
			ID:     "types",
			Detail: fmt.Sprintf("importer だけ: [%s] / 参照だけ: [%s]", strings.Join(onlyImported, ","), strings.Join(onlyRef, ",")),
		})
	}

	codes := make(map[[2]string]int, len(chart))
	for _, r := range chart {
		codes[[2]string{r.AttackType, r.DefenseType}] = r.Code
	}
	var common []string
	for _, id := range ref.Types {
		if importedSet[id] {
			common = append(common, id)
		}
	}
	sort.Strings(common)
	for _, atk := range common {
		for _, def := range common {
			got, ok := codes[[2]string{atk, def}]
			if !ok {
				got = neutralCode
			}
			want := ref.Effectiveness[atk][def]
			if got != want {
				findings = append(findings, Finding{
					Kind:   KindTypeChartReferenceMismatch,
					ID:     atk + ">" + def,
					Detail: fmt.Sprintf("importer %d / 参照 %d", got, want),
				})
			}
		}
	}
	return findings
}

// ReferenceTypeChartDefaultPath は data ディレクトリから見た参照の相性表の既定の場所。
// リポジトリの配置(data/ と testdata/golden/)と importer のイメージの配置(/app/data と
// /app/testdata/golden。services/pokedex/Dockerfile)のどちらでも data の隣に testdata がある。
func ReferenceTypeChartDefaultPath(dataDir string) string {
	return filepath.Join(dataDir, "..", "testdata", "golden", "typechart.json")
}
