package importer

import "encoding/json"

// スナップショット・設定ファイルの入力型(ADR-0101 §3・§12)。JSON キーは ADR §3 の表のとおり。

// BaseStats は種族値(calc・Showdown 共通の形)。
type BaseStats struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

// CalcSnapshot は @smogon/calc から抽出した正規化スナップショット。
type CalcSnapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Source        string                    `json:"source"`
	Version       string                    `json:"version"`
	Generation    int                       `json:"generation"`
	Types         []string                  `json:"types"`
	TypeChart     map[string]map[string]int `json:"typeChart"`
	Species       []CalcSpecies             `json:"species"`
	Moves         []CalcMove                `json:"moves"`
	Items         []string                  `json:"items"`
	Abilities     []string                  `json:"abilities"`
}

// CalcSpecies は calc の種族1件。
type CalcSpecies struct {
	Name      string    `json:"name"`
	Types     []string  `json:"types"`
	BaseStats BaseStats `json:"baseStats"`
}

// CalcMove は calc の技1件。Type/Category は calc が省略すると空文字になる。
type CalcMove struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Category  string `json:"category"`
	BasePower int    `json:"basePower"`
	Priority  int    `json:"priority"`
}

// ShowdownSnapshot は Showdown の champions mod から抽出した正規化スナップショット。
type ShowdownSnapshot struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Source        string              `json:"source"`
	Version       string              `json:"version"`
	Mod           string              `json:"mod"`
	Species       []ShowdownSpecies   `json:"species"`
	Moves         []ShowdownMove      `json:"moves"`
	Items         []ShowdownItem      `json:"items"`
	Abilities     []ShowdownAbility   `json:"abilities"`
	Learnsets     map[string][]string `json:"learnsets"`
}

// ShowdownSpecies は Showdown の種族1件。
type ShowdownSpecies struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Num           int               `json:"num"`
	BaseSpecies   string            `json:"baseSpecies"`
	Forme         string            `json:"forme"`
	BaseForme     string            `json:"baseForme"`
	Types         []string          `json:"types"`
	BaseStats     BaseStats         `json:"baseStats"`
	Abilities     map[string]string `json:"abilities"`
	RequiredItem  string            `json:"requiredItem"`
	FormeOrder    []string          `json:"formeOrder"`
	IsNonstandard *string           `json:"isNonstandard"`
}

// ShowdownMove は Showdown の技1件。Accuracy 0 は必中(calc の `accuracy: true`)。
type ShowdownMove struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Category      string  `json:"category"`
	BasePower     int     `json:"basePower"`
	Accuracy      int     `json:"accuracy"`
	PP            int     `json:"pp"`
	Priority      int     `json:"priority"`
	IsNonstandard *string `json:"isNonstandard"`
}

// ShowdownItem は Showdown の持ち物1件。
type ShowdownItem struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	IsNonstandard *string `json:"isNonstandard"`
}

// ShowdownAbility は Showdown の特性1件。
type ShowdownAbility struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	IsNonstandard *string `json:"isNonstandard"`
}

// PokeAPISnapshot は PokeAPI の CSV から抽出した名前だけのスナップショット。
type PokeAPISnapshot struct {
	SchemaVersion int           `json:"schemaVersion"`
	Source        string        `json:"source"`
	Version       string        `json:"version"`
	Species       []PokeAPIName `json:"species"`
	Forms         []PokeAPIName `json:"forms"`
	Moves         []PokeAPIName `json:"moves"`
	Items         []PokeAPIName `json:"items"`
	Abilities     []PokeAPIName `json:"abilities"`
	Types         []PokeAPIName `json:"types"`
}

// PokeAPIName は PokeAPI の1エントリ(スラッグ + 言語別の名前)。
type PokeAPIName struct {
	Slug  string            `json:"slug"`
	Names map[string]string `json:"names"`
}

// NameOverrides は日本語名の人手の上書き(data/local/name_ja_overrides.json。実データなので Git 管理外)。
type NameOverrides struct {
	SchemaVersion int               `json:"schemaVersion"`
	Species       map[string]string `json:"species"`
	Moves         map[string]string `json:"moves"`
	Items         map[string]string `json:"items"`
	Abilities     map[string]string `json:"abilities"`
	Types         map[string]string `json:"types"`
}

// EffectsFile は data/importer/effects.json(効果定義。人が管理する正)。
type EffectsFile struct {
	SchemaVersion int                        `json:"schemaVersion"`
	Items         map[string]json.RawMessage `json:"items"`
	Abilities     map[string]json.RawMessage `json:"abilities"`
}

// RegulationsFile は data/importer/regulations.json。
type RegulationsFile struct {
	SchemaVersion int             `json:"schemaVersion"`
	Regulations   []RegulationDef `json:"regulations"`
}

// RegulationDef は1レギュレーションの定義。
type RegulationDef struct {
	ID          string `json:"id"`
	NameJa      string `json:"nameJa"`
	IsDefault   bool   `json:"isDefault"`
	StartsOn    string `json:"startsOn"`
	EndsOn      string `json:"endsOn"`
	ShowdownMod string `json:"showdownMod"`
}

// Config は data/importer/config.json(取得元の版・除外・日本語名の言語優先順)。
type Config struct {
	SchemaVersion      int               `json:"schemaVersion"`
	Sources            map[string]string `json:"sources"`
	ExcludeCalcSpecies []string          `json:"excludeCalcSpecies"`
	ExcludeTypes       []string          `json:"excludeTypes"`
	NameJaLanguages    []string          `json:"nameJaLanguages"`
}

// Input は LoadInput が読み込む一式。
type Input struct {
	Calc        CalcSnapshot
	Showdown    ShowdownSnapshot
	PokeAPI     PokeAPISnapshot
	Overrides   NameOverrides
	Effects     EffectsFile
	Regulations RegulationsFile
	Config      Config
}
