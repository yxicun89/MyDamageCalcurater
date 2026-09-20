package engine

import (
	"errors"
	"fmt"
)

// Species はポケモンの種族データ(マスタから解決済み)。
type Species struct {
	Key       string   // {図鑑番号4桁}-{フォルム3桁} 例 0445-000
	DexNo     int      //
	Form      int      //
	NameJa    string   //
	Types     []Type   // 1〜2個
	BaseStats Stats    //
	Abilities []string // 特性 ID(参考。計算では Individual.Ability を使う)
}

// Move は技データ(マスタから解決済み)。
type Move struct {
	ID       string
	NameJa   string
	Type     Type
	Category MoveCategory
	Power    int // 威力。0 は変化技/固定ダメージ
	Priority int
}

// Item は持ち物データ。Effect はダメージ補正の定義(マスタから解決)。nil は補正なし。
type Item struct {
	ID     string
	NameJa string
	Effect *ItemEffect
}

// Ability は特性データ。Effect はダメージ補正の定義(マスタから解決)。nil は補正なし。
type Ability struct {
	ID     string
	NameJa string
	Effect *AbilityEffect
}

// Nature は性格補正。Plus のステータスが +10%、Minus が -10%。
// 無補正(まじめ等)は Plus/Minus とも空("")。HP には補正がかからない。
type Nature struct {
	Plus  StatKey
	Minus StatKey
}

// NatureNeutral は無補正の性格。
var NatureNeutral = Nature{}

// IsNeutral は補正なし(実質等倍)かどうかを返す。
// Plus と Minus が同一のときも打ち消しあって等倍になる。
//
// 補正倍率(1.1/1.0/0.9)を float で持つと「4096基準の固定小数・float 近似禁止」
// (CLAUDE.md)に反する経路を作りかねないため、実際の適用は整数演算の
// applyNature(stats.go)に一本化している。
func (n Nature) IsNeutral() bool {
	return n.Plus == n.Minus
}

// rankStatKeys はランク補正を持つステータス(HP を除く)。
var rankStatKeys = []StatKey{StatAtk, StatDef, StatSpA, StatSpD, StatSpe}

// Ranks はランク補正(-6..+6)。HP は持たない。
type Ranks struct {
	Atk int
	Def int
	SpA int
	SpD int
	Spe int
}

// Get は StatKey に対応するランクを返す(HP は常に 0)。
func (r Ranks) Get(k StatKey) int {
	switch k {
	case StatAtk:
		return r.Atk
	case StatDef:
		return r.Def
	case StatSpA:
		return r.SpA
	case StatSpD:
		return r.SpD
	case StatSpe:
		return r.Spe
	}
	return 0
}

// Screens は壁(リフレクター/ひかりのかべ/オーロラベール)。
type Screens struct {
	Reflect     bool
	LightScreen bool
	AuroraVeil  bool
}

// Field は場の状態。
type Field struct {
	Weather         Weather
	Terrain         Terrain
	AttackerScreens Screens
	DefenderScreens Screens
}

// Individual は計算に使う個体。SP と性格から実数値を導出する。
type Individual struct {
	Species  Species
	Level    int // 0 のときは DefaultLevel(50)扱い
	Nature   Nature
	Ability  Ability
	Item     *Item // なしは nil
	SP       Stats // 各 0..32、合計 <= 66
	Ranks    Ranks
	TeraType Type // テラスタルなしは TypeNone
	Status   Status
}

// EffectiveLevel は Level が未設定(0)なら DefaultLevel を返す。
func (in Individual) EffectiveLevel() int {
	if in.Level <= 0 {
		return DefaultLevel
	}
	return in.Level
}

// Validate は個体の妥当性を検証する。SP の範囲(各 0..32・合計 <= 66)、
// ランク(-6..6)、性格(HP に補正なし)を確認する。
func (in Individual) Validate() error {
	if in.Level != 0 && in.Level != DefaultLevel {
		return fmt.Errorf("レベルは %d 固定: %d", DefaultLevel, in.Level)
	}
	for _, k := range AllStatKeys {
		if in.Species.BaseStats.Get(k) < 0 {
			return fmt.Errorf("種族値 %s は負にできない", k)
		}
		v := in.SP.Get(k)
		if v < 0 || v > MaxSPPerStat {
			return fmt.Errorf("SP %s は 0..%d の範囲外: %d", k, MaxSPPerStat, v)
		}
	}
	if sum := in.SP.Sum(); sum > MaxSPTotal {
		return fmt.Errorf("SP 合計が上限 %d を超過: %d", MaxSPTotal, sum)
	}
	for _, k := range rankStatKeys {
		rv := in.Ranks.Get(k)
		if rv < -6 || rv > 6 {
			return fmt.Errorf("ランク補正 %s は -6..6 の範囲外: %d", k, rv)
		}
	}
	if in.Nature.Plus == StatHP || in.Nature.Minus == StatHP {
		return errors.New("性格補正は HP に適用できない")
	}
	if len(in.Species.Types) == 0 || len(in.Species.Types) > 2 {
		return fmt.Errorf("タイプは1〜2個: %d", len(in.Species.Types))
	}
	return nil
}
