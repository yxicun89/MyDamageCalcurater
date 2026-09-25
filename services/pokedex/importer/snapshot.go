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
	// Natures は性格(ADR-0105 §4)。無補正は Plus==Minus(calc は無補正を「同じ能力の上昇と下降」で表す。ID を持たない)。
	Natures []CalcNature `json:"natures"`
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
// Learnsets は種族ID→技ID→その技を学習できる学習元のうち最大の世代番号(ADR-0103 §7)。
// 例えば学習元の符号が "9M"(第9世代マシン)・"7L12"(第7世代レベル12)・"8E"(第8世代タマゴ技)なら、
// 先頭の数字が世代。同じ技に複数の学習元があれば最大値を採る(フォーマットの minSourceGen 以上の
// 学習元が1つでもあれば学習可能に足りるため)。
type ShowdownSnapshot struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Source        string                    `json:"source"`
	Version       string                    `json:"version"`
	Mod           string                    `json:"mod"`
	Species       []ShowdownSpecies         `json:"species"`
	Moves         []ShowdownMove            `json:"moves"`
	Items         []ShowdownItem            `json:"items"`
	Abilities     []ShowdownAbility         `json:"abilities"`
	Learnsets     map[string]map[string]int `json:"learnsets"`
	// Natures は性格(ADR-0105 §4)。補正の正。無補正は Plus/Minus とも省略(空文字)。
	Natures []ShowdownNature `json:"natures"`
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
	// Prevo は進化前の種族の Showdown 名(無ければ空)。習得技の継承(ADR-0103 §7)に使う。
	Prevo string `json:"prevo"`
}

// ShowdownMove は Showdown の技1件。Accuracy 0 は必中(calc の `accuracy: true`)。
// Self/Secondary/Secondaries は追加効果(命中時のランク変化。ADR-0107 決定6)。
// 取得元の表現のまま持つ(ID化・正準化は convert で行う)。
type ShowdownMove struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Type          string             `json:"type"`
	Category      string             `json:"category"`
	BasePower     int                `json:"basePower"`
	Accuracy      int                `json:"accuracy"`
	PP            int                `json:"pp"`
	Priority      int                `json:"priority"`
	IsNonstandard *string            `json:"isNonstandard"`
	Self          *ShowdownBoosts    `json:"self"`
	Secondary     *ShowdownSecondary `json:"secondary"`
	// Secondaries は取得元の配列そのまま(要素数は2以上のこともある)。boosts を伴う要素の件数を
	// 数えるのは convert 側(ID化・正準化は Go 側という方針。ADR-0101 §3)。
	// boosts を持たない要素(状態異常・ひるみ等)は決定3・6 の対象外(ADR-0107 2026-09-23 追記)。
	Secondaries []ShowdownSecondary `json:"secondaries"`
	// Mechanism は技の機構(多段・固定ダメージ・威力変動 等)の判定材料(ADR-0121)。必須:
	// 無い(古い取得物)と全技が「通常の技」として黙って分類されるので、デコードで拒否する。
	Mechanism *ShowdownMoveMechanism `json:"mechanism"`
}

// ShowdownMoveMechanism は技の機構の判定材料。Showdown の技データの表現のまま持つ
// (分類は convert_move_mechanisms.go。ADR-0121)。
//   - Multihit: null・回数(数値)・[最小, 最大] のいずれか
//   - Damage: null・固定ダメージ(数値)・"level"(使用者のレベルと同じ)のいずれか
//   - OHKO: null・true・タイプ名(そのタイプには効かない一撃必殺)のいずれか
//   - Hooks: 技のデータオブジェクトが持つ関数のプロパティ名の昇順
//   - FieldConditions: 天候・フィールドのハンドラがこの技の ID を名指ししている箇所("<状態ID>.<ハンドラ名>" の昇順)
type ShowdownMoveMechanism struct {
	Multihit                 json.RawMessage `json:"multihit"`
	Damage                   json.RawMessage `json:"damage"`
	OHKO                     json.RawMessage `json:"ohko"`
	WillCrit                 bool            `json:"willCrit"`
	OverrideOffensiveStat    string          `json:"overrideOffensiveStat"`
	OverrideOffensivePokemon string          `json:"overrideOffensivePokemon"`
	OverrideDefensiveStat    string          `json:"overrideDefensiveStat"`
	IgnoreDefensive          bool            `json:"ignoreDefensive"`
	Hooks                    []string        `json:"hooks"`
	FieldConditions          []string        `json:"fieldConditions"`
}

// ShowdownBoosts は技のトップレベル self.boosts(命中すれば必ず発動)。
type ShowdownBoosts struct {
	Boosts map[string]int `json:"boosts"`
}

// ShowdownSecondary は技の secondary(確率つきの追加効果)。
// Self があれば使用者自身のランク変化、Boosts があれば対象のランク変化(ADR-0107「調査」)。
type ShowdownSecondary struct {
	Chance int             `json:"chance"`
	Self   *ShowdownBoosts `json:"self"`
	Boosts map[string]int  `json:"boosts"`
}

// ShowdownItem は Showdown の持ち物1件。
type ShowdownItem struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	IsNonstandard *string `json:"isNonstandard"`
	// Hooks はその持ち物のデータオブジェクトが持つ、on で始まる関数のプロパティ名の昇順(無ければ空。ADR-0103 §6)。
	Hooks []string `json:"hooks"`
}

// ShowdownAbility は Showdown の特性1件。
type ShowdownAbility struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	IsNonstandard *string `json:"isNonstandard"`
	// Hooks は ShowdownItem.Hooks と同じ(ADR-0103 §6)。
	Hooks []string `json:"hooks"`
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
	// Natures は性格の日本語名(ADR-0105 §4)。
	Natures []PokeAPIName `json:"natures"`
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
	Natures       map[string]string `json:"natures"`
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
// InheritFromPrevo は習得技の解決(ADR-0103 §7)で進化前(prevo)の学習元をたどるかどうか。
// Showdown の `learnsetParent` は mod によって進化前をたどるかが変わる(champions は常にたどらない。
// §10 の実データ確認で判明)ため、Go にモード名をハードコードせずレギュレーションの定義に持たせる。
// M-C は false(自分の学習元、無ければ基本種の学習元を1段だけ)。true なら進化前も何段でもたどる。
// MinSourceGen(学習元として認める最小の世代番号。Showdown の TeamValidator の `minSourceGen` に対応)
// は InheritFromPrevo の値に関わらず常に効く(自分の学習元にも適用する絞り込みのため)。
type RegulationDef struct {
	ID               string `json:"id"`
	NameJa           string `json:"nameJa"`
	IsDefault        bool   `json:"isDefault"`
	StartsOn         string `json:"startsOn"`
	EndsOn           string `json:"endsOn"`
	ShowdownMod      string `json:"showdownMod"`
	InheritFromPrevo bool   `json:"inheritFromPrevo"`
	MinSourceGen     int    `json:"minSourceGen"`
}

// Config は data/importer/config.json(取得元の版・除外・日本語名の言語優先順)。
type Config struct {
	SchemaVersion      int               `json:"schemaVersion"`
	Sources            map[string]string `json:"sources"`
	ExcludeCalcSpecies []string          `json:"excludeCalcSpecies"`
	ExcludeTypes       []string          `json:"excludeTypes"`
	NameJaLanguages    []string          `json:"nameJaLanguages"`
	// Reconcile は照合の設定(ADR-0103 §5・§6・§9)。任意(無ければ Convert 単体は従来どおり動く。
	// Reconcile 関数は必須にする)。有れば厳格に検証する。
	Reconcile *ReconcileConfig `json:"reconcile"`
}

// ReconcileConfig は Config.Reconcile(ADR-0103 §11)。
type ReconcileConfig struct {
	EffectHooks []string `json:"effectHooks"`
	Verdicts    Verdicts `json:"verdicts"`
}

// Verdicts は P2-1c の裁定の反映の確認に使う値(ADR-0103 §5)。
type Verdicts struct {
	Basis map[string]string `json:"basis"`
	Moves MoveVerdicts      `json:"moves"`
}

// MoveVerdicts は技の使用可否の裁定の3区分(ADR-0103 §5)。
type MoveVerdicts struct {
	CalcOnlyExcluded     VerdictCount `json:"calcOnlyExcluded"`
	ShowdownOnlyIncluded VerdictCount `json:"showdownOnlyIncluded"`
	StatusTypeMismatch   VerdictCount `json:"statusTypeMismatch"`
}

// VerdictCount は1区分の期待件数と ID 集合のハッシュ(ADR-0103 §5)。
type VerdictCount struct {
	Count     int    `json:"count"`
	IDsSHA256 string `json:"idsSha256"`
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

	// ReferenceTypeChart は照合(Reconcile)が比べる参照の相性表(issue #280・ADR-0118)。
	// LoadInput は読まない(data の外にある)。呼び出し側が LoadReferenceTypeChart で読んで入れる。
	// Reconcile では必須、Convert では使わない。
	ReferenceTypeChart *ReferenceTypeChart
}
