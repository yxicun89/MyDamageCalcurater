package engine

// 固定パラメータ(CLAUDE.md ドメイン規約)。
const (
	// DefaultLevel は対戦レベル。ポケモンチャンピオンズは Lv50 固定。
	DefaultLevel = 50
	// FixedIV は個体値。全ステータス 31 固定。
	FixedIV = 31
	// HPStatOffset は HP 実数値の式 `種族値 + HPStatOffset + SP` の定数項。
	// Lv50・個体値31 で標準式を展開した値(CLAUDE.md ドメイン規約)。
	// 標準式 floor((2×種族値+個体値+努力値/4)×Lv/100)+Lv+10 に SP=0(努力値0)を
	// 入れると 種族値 + 50(Lv) + 10 + 15(=floor(31/2)) = 種族値 + 75。
	HPStatOffset = 75
	// OtherStatOffset は HP 以外の実数値の式 `floor((種族値 + OtherStatOffset + SP) × 性格補正)` の定数項。
	// 標準式 floor(floor((2×種族値+個体値+努力値/4)×Lv/100)+5)×性格補正 に SP=0 を
	// 入れると 種族値 + 5 + 15(=floor(31/2)) = 種族値 + 20。
	OtherStatOffset = 20
	// MaxSPPerStat は 1ステータスあたりの能力ポイント上限。
	MaxSPPerStat = 32
	// MaxSPTotal は能力ポイント合計の上限。
	MaxSPTotal = 66
)

// Format は対戦形式。ダブル固有の補正を後から足せるよう最初から持たせる。
type Format string

const (
	FormatSingle Format = "single"
	FormatDouble Format = "double"
)

// StatKey は6ステータスのキー。Showdown 規約(@smogon/calc 照合のため)。
type StatKey string

const (
	StatHP  StatKey = "hp"
	StatAtk StatKey = "atk"
	StatDef StatKey = "def"
	StatSpA StatKey = "spa"
	StatSpD StatKey = "spd"
	StatSpe StatKey = "spe"
)

// Type はポケモン/技のタイプ。空文字("")は「タイプなし」を表す。
type Type string

const (
	TypeNone     Type = ""
	TypeNormal   Type = "normal"
	TypeFire     Type = "fire"
	TypeWater    Type = "water"
	TypeElectric Type = "electric"
	TypeGrass    Type = "grass"
	TypeIce      Type = "ice"
	TypeFighting Type = "fighting"
	TypePoison   Type = "poison"
	TypeGround   Type = "ground"
	TypeFlying   Type = "flying"
	TypePsychic  Type = "psychic"
	TypeBug      Type = "bug"
	TypeRock     Type = "rock"
	TypeGhost    Type = "ghost"
	TypeDragon   Type = "dragon"
	TypeDark     Type = "dark"
	TypeSteel    Type = "steel"
	TypeFairy    Type = "fairy"
)

// MoveCategory は技の分類。
type MoveCategory string

const (
	CategoryPhysical MoveCategory = "physical"
	CategorySpecial  MoveCategory = "special"
	CategoryStatus   MoveCategory = "status"
)

// Weather は天候。
type Weather string

const (
	WeatherNone Weather = "none"
	WeatherSun  Weather = "sun"
	WeatherRain Weather = "rain"
	WeatherSand Weather = "sand"
	WeatherSnow Weather = "snow"
)

// Terrain はフィールド状態。
type Terrain string

const (
	TerrainNone     Terrain = "none"
	TerrainElectric Terrain = "electric"
	TerrainGrassy   Terrain = "grassy"
	TerrainMisty    Terrain = "misty"
	TerrainPsychic  Terrain = "psychic"
)

// Status は状態異常。
type Status string

const (
	StatusNone        Status = "none"
	StatusBurn        Status = "burn"
	StatusParalysis   Status = "paralysis"
	StatusPoison      Status = "poison"
	StatusBadlyPoison Status = "badly_poison"
	StatusSleep       Status = "sleep"
	StatusFreeze      Status = "freeze"
)

// Stats は6ステータスの値。種族値・SP・実数値に共用する。
type Stats struct {
	HP  int
	Atk int
	Def int
	SpA int
	SpD int
	Spe int
}

// Get は StatKey に対応する値を返す。未知のキーは 0。
func (s Stats) Get(k StatKey) int {
	switch k {
	case StatHP:
		return s.HP
	case StatAtk:
		return s.Atk
	case StatDef:
		return s.Def
	case StatSpA:
		return s.SpA
	case StatSpD:
		return s.SpD
	case StatSpe:
		return s.Spe
	}
	return 0
}

// WithStat は StatKey の値を v にした新しい Stats を返す(元は変更しない)。
func (s Stats) WithStat(k StatKey, v int) Stats {
	switch k {
	case StatHP:
		s.HP = v
	case StatAtk:
		s.Atk = v
	case StatDef:
		s.Def = v
	case StatSpA:
		s.SpA = v
	case StatSpD:
		s.SpD = v
	case StatSpe:
		s.Spe = v
	}
	return s
}

// Sum は6ステータスの合計。SP 合計上限の判定に使う。
func (s Stats) Sum() int {
	return s.HP + s.Atk + s.Def + s.SpA + s.SpD + s.Spe
}

// AllStatKeys は6ステータスのキー一覧(反復用)。
var AllStatKeys = []StatKey{StatHP, StatAtk, StatDef, StatSpA, StatSpD, StatSpe}
