package domain

// お気に入り(手動ピン留め)の内容の検証と正規化(ADR-0227・ADR-0228。旧 adapters/httpapi/favorites.go)。
// record-svc は pokedex-svc に依存しないので、ID がマスタに実在するかは見ない(形式・範囲だけ検証する)。
// 保存する内容は既定値を補った正規化済みの JSON(キーの順序が固定。同じ内容なら同じバイト列)で、
// store はそのバイト列のハッシュで重複を判定する。label・snapshot の中身は利用者の自由入力を含むのでログに出さない(ADR-0209 §3)。
//
// *Input の JSON のキーは契約(CreateFavoriteRequest・CalcRequest)と同じで、adapters が本文を厳格に読み込む先になる
// (契約に無いキーは adapters の厳格デコードで unknown_field)。*Snapshot の JSON は record DB に保存する形そのもの
// (フィールドの宣言順がキーの順。変えると既存行のハッシュと一致しなくなる)。

import (
	"encoding/json"
	"regexp"
	"unicode/utf8"

	"example.com/pokecalc/engine"
)

// SP・レベルの上限は engine の定数(calc-svc が使う正)を参照し、ここで二重に定義しない。
// ランクの上限だけは engine に定数が無い(engine は ±6 をリテラルで持つ)ので、ここに置く(ADR-0228 §3)。
const (
	maxFavoriteLabelRunes = 30
	maxRank               = 6
	// MaxFavoriteSnapshotBytes は正規化後の snapshot の上限(ADR-0228 §3。正当な入力は 2KB に満たない)。
	MaxFavoriteSnapshotBytes = 4096
	// 対戦の状態の上限(契約の CalcBattleState.hits の maximum)。
	maxBattleStateHits = 10
)

var speciesKeyPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{3}$`)

// IsSpeciesKey は s が {図鑑番号4桁}-{フォルム3桁} の形式かを返す。
func IsSpeciesKey(s string) bool { return speciesKeyPattern.MatchString(s) }

// Enums は契約の列挙の判定と、省略時の値(どちらも契約が正。adapters が生成型の Valid() と定数から作る。ADR-0252 と同じ考え方)。
type Enums struct {
	Status, Type, Format, Weather, Terrain func(string) bool
	StatusNone, WeatherNone, TerrainNone   string
}

// FavoriteInput・IndividualInput は本文の受け口。必須項目の欠落を見分けるためポインタで受ける。
type FavoriteInput struct {
	Label      *string          `json:"label"`
	Individual *IndividualInput `json:"individual"`
	Calc       *CalcInput       `json:"calc"`
}

// CalcInput は契約の CalcRequest の受け口(ADR-0228)。欠落を見分けるためポインタで受ける。
type CalcInput struct {
	Format   *string          `json:"format"`
	Attacker *IndividualInput `json:"attacker"`
	Defender *IndividualInput `json:"defender"`
	MoveID   *string          `json:"moveId"`
	Field    *FieldInput      `json:"field"`
	Options  *OptionsInput    `json:"options"`
	// BattleState は対戦の状態(残り HP・多段の回数。ADR-0144)。省略・null・{} は省略のまま保存する。
	BattleState *BattleStateInput `json:"battleState"`
}

// BattleStateInput は契約の CalcBattleState の受け口。値域は契約の minimum / maximum と同じ(最大 HP との照合はマスタが要るのでしない。
// 最大を超える残り HP は計算するとき calc-svc が invalid_input にする)。
type BattleStateInput struct {
	AttackerCurrentHP *int `json:"attackerCurrentHp"`
	DefenderCurrentHP *int `json:"defenderCurrentHp"`
	Hits              *int `json:"hits"`
}

type FieldInput struct {
	Weather         *string       `json:"weather"`
	Terrain         *string       `json:"terrain"`
	AttackerScreens *ScreensInput `json:"attackerScreens"`
	DefenderScreens *ScreensInput `json:"defenderScreens"`
}

type ScreensInput struct {
	Reflect     *bool `json:"reflect"`
	LightScreen *bool `json:"lightScreen"`
	AuroraVeil  *bool `json:"auroraVeil"`
}

type OptionsInput struct {
	Critical *bool `json:"critical"`
}

type IndividualInput struct {
	SpeciesKey *string    `json:"speciesKey"`
	Level      *int       `json:"level"`
	NatureID   *string    `json:"natureId"`
	AbilityID  *string    `json:"abilityId"`
	ItemID     *string    `json:"itemId"`
	Sp         *SPInput   `json:"sp"`
	Ranks      *RankInput `json:"ranks"`
	TeraType   *string    `json:"teraType"`
	Status     *string    `json:"status"`
}

type SPInput struct {
	Hp  *int `json:"hp"`
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	Spa *int `json:"spa"`
	Spd *int `json:"spd"`
	Spe *int `json:"spe"`
}

// RankInput はランク。保存では6キー(HP を除く5キー)すべてを持つ(省略は0)。
type RankInput struct {
	Atk *int `json:"atk"`
	Def *int `json:"def"`
	Spa *int `json:"spa"`
	Spd *int `json:"spd"`
	Spe *int `json:"spe"`
}

// FavoriteSnapshot・IndividualSnapshot は保存する正規化済みの形。フィールドの宣言順がそのまま
// JSON のキー順になる(同じ内容なら同じバイト列)。
type FavoriteSnapshot struct {
	Label      *string            `json:"label"`
	Individual IndividualSnapshot `json:"individual"`
	// Calc は計算の入力全体(ADR-0228)。無いときはキーごと出さない(既存行のバイト列・ハッシュを変えない)。
	Calc *CalcSnapshot `json:"calc,omitempty"`
}

// CalcSnapshot は calc の正規化済みの形。フィールドの宣言順が契約のプロパティ順(= JSON のキー順)。
type CalcSnapshot struct {
	Format   string             `json:"format"`
	Attacker IndividualSnapshot `json:"attacker"`
	Defender IndividualSnapshot `json:"defender"`
	MoveID   string             `json:"moveId"`
	Field    FieldSnapshot      `json:"field"`
	Options  OptionsSnapshot    `json:"options"`
	// BattleState は対戦の状態(ADR-0144)。無いときはキーごと出さない(既存行のバイト列・ハッシュを変えない)。
	BattleState *BattleStateSnapshot `json:"battleState,omitempty"`
}

// BattleStateSnapshot は battleState の正規化済みの形(指定したキーだけを持つ。フィールドの宣言順が JSON のキー順)。
type BattleStateSnapshot struct {
	AttackerCurrentHP *int `json:"attackerCurrentHp,omitempty"`
	DefenderCurrentHP *int `json:"defenderCurrentHp,omitempty"`
	Hits              *int `json:"hits,omitempty"`
}

type FieldSnapshot struct {
	Weather         string          `json:"weather"`
	Terrain         string          `json:"terrain"`
	AttackerScreens ScreensSnapshot `json:"attackerScreens"`
	DefenderScreens ScreensSnapshot `json:"defenderScreens"`
}

type ScreensSnapshot struct {
	Reflect     bool `json:"reflect"`
	LightScreen bool `json:"lightScreen"`
	AuroraVeil  bool `json:"auroraVeil"`
}

type OptionsSnapshot struct {
	Critical bool `json:"critical"`
}

type IndividualSnapshot struct {
	SpeciesKey string     `json:"speciesKey"`
	Level      int        `json:"level"`
	NatureID   string     `json:"natureId"`
	AbilityID  string     `json:"abilityId,omitempty"`
	ItemID     string     `json:"itemId,omitempty"`
	Sp         SPSnapshot `json:"sp"`
	Ranks      RankValues `json:"ranks"`
	TeraType   string     `json:"teraType,omitempty"`
	Status     string     `json:"status"`
}

type SPSnapshot struct {
	Hp  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	Spa int `json:"spa"`
	Spd int `json:"spd"`
	Spe int `json:"spe"`
}

type RankValues struct {
	Atk int `json:"atk"`
	Def int `json:"def"`
	Spa int `json:"spa"`
	Spd int `json:"spd"`
	Spe int `json:"spe"`
}

// NormalizeFavorite は本文を検証し、既定値を補った正規化済みの形にする。
func NormalizeFavorite(req FavoriteInput, enums Enums) (FavoriteSnapshot, error) {
	var out FavoriteSnapshot
	if req.Label != nil && *req.Label != "" {
		if n := utf8.RuneCountInString(*req.Label); n > maxFavoriteLabelRunes {
			return out, Errorf(CodeInvalidInput, "label は%d文字まで(%d文字)", maxFavoriteLabelRunes, n)
		}
		out.Label = req.Label
	}
	ind, err := NormalizeIndividual(req.Individual, enums)
	if err != nil {
		return out, err
	}
	out.Individual = ind
	if req.Calc != nil {
		calc, err := NormalizeCalc(req.Calc, enums)
		if err != nil {
			return out, err
		}
		out.Calc = &calc
	}
	return out, nil
}

// EncodeFavorite は正規化済みの形を保存するバイト列にする。MaxFavoriteSnapshotBytes を超えたら invalid_input。
// JSON 化そのものの失敗(通常は起きない)は domain.Error ではない失敗として返す(呼び出し側が想定外の失敗として扱う)。
func EncodeFavorite(snap FavoriteSnapshot) ([]byte, error) {
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	if len(raw) > MaxFavoriteSnapshotBytes {
		return nil, Errorf(CodeInvalidInput, "お気に入りの内容が大きすぎる(%dバイト以下)", MaxFavoriteSnapshotBytes)
	}
	return raw, nil
}

// NormalizeIndividual は individual と calc の attacker / defender に共通の検証・正規化(ADR-0228 §3)。
func NormalizeIndividual(in *IndividualInput, enums Enums) (IndividualSnapshot, error) {
	var out IndividualSnapshot
	if in == nil {
		return out, Errorf(CodeInvalidInput, "individual が無い")
	}
	// 未知の列挙値は invalid_enum(範囲の検査より先に見る)。
	status := enums.StatusNone
	if in.Status != nil {
		if !enums.Status(*in.Status) {
			return out, Errorf(CodeInvalidEnum, "status が未知の値")
		}
		status = *in.Status
	}
	tera := ""
	if in.TeraType != nil {
		if !enums.Type(*in.TeraType) {
			return out, Errorf(CodeInvalidEnum, "teraType が未知の値")
		}
		tera = *in.TeraType
	}

	if in.SpeciesKey == nil || !speciesKeyPattern.MatchString(*in.SpeciesKey) {
		return out, Errorf(CodeInvalidInput, "speciesKey は {図鑑番号4桁}-{フォルム3桁} の形式でなければならない")
	}
	if in.NatureID == nil || *in.NatureID == "" {
		return out, Errorf(CodeInvalidInput, "natureId が無い")
	}
	level := engine.DefaultLevel
	if in.Level != nil {
		level = *in.Level
	}
	if level != engine.DefaultLevel {
		return out, Errorf(CodeInvalidInput, "level は%dでなければならない(%d)", engine.DefaultLevel, level)
	}
	sp, err := normalizeSP(in.Sp)
	if err != nil {
		return out, err
	}
	ranks, err := normalizeRanks(in.Ranks)
	if err != nil {
		return out, err
	}

	out = IndividualSnapshot{
		SpeciesKey: *in.SpeciesKey, Level: level, NatureID: *in.NatureID,
		AbilityID: deref(in.AbilityID), ItemID: deref(in.ItemID),
		Sp: sp, Ranks: ranks, TeraType: tera, Status: status,
	}
	return out, nil
}

// NormalizeCalc は calc を検証し、既定値を補った形にする。マスタとは照合しない(ADR-0228 §3)。
func NormalizeCalc(in *CalcInput, enums Enums) (CalcSnapshot, error) {
	var out CalcSnapshot
	if in.Format == nil || !enums.Format(*in.Format) {
		return out, Errorf(CodeInvalidEnum, "calc.format が無いか未知の値")
	}
	weather, terrain := enums.WeatherNone, enums.TerrainNone
	var fieldIn FieldInput
	if in.Field != nil {
		fieldIn = *in.Field
	}
	if fieldIn.Weather != nil {
		if !enums.Weather(*fieldIn.Weather) {
			return out, Errorf(CodeInvalidEnum, "calc.field.weather が未知の値")
		}
		weather = *fieldIn.Weather
	}
	if fieldIn.Terrain != nil {
		if !enums.Terrain(*fieldIn.Terrain) {
			return out, Errorf(CodeInvalidEnum, "calc.field.terrain が未知の値")
		}
		terrain = *fieldIn.Terrain
	}
	if in.Attacker == nil || in.Defender == nil {
		return out, Errorf(CodeInvalidInput, "calc.attacker と calc.defender が必要")
	}
	if in.MoveID == nil || *in.MoveID == "" {
		return out, Errorf(CodeInvalidInput, "calc.moveId が無い")
	}
	attacker, err := NormalizeIndividual(in.Attacker, enums)
	if err != nil {
		return out, err
	}
	defender, err := NormalizeIndividual(in.Defender, enums)
	if err != nil {
		return out, err
	}
	out = CalcSnapshot{
		Format: *in.Format, Attacker: attacker, Defender: defender, MoveID: *in.MoveID,
		Field: FieldSnapshot{
			Weather: weather, Terrain: terrain,
			AttackerScreens: normalizeScreens(fieldIn.AttackerScreens),
			DefenderScreens: normalizeScreens(fieldIn.DefenderScreens),
		},
	}
	if in.Options != nil && in.Options.Critical != nil {
		out.Options.Critical = *in.Options.Critical
	}
	if out.BattleState, err = normalizeBattleState(in.BattleState); err != nil {
		return out, err
	}
	return out, nil
}

// normalizeBattleState は battleState を契約(CalcBattleState)と同じ値域で検証する。残り HP は 1 以上(0 は満タンの意味にしない)、
// 回数は 1..10。何も指定が無い({})は省略と同じ(nil)。
func normalizeBattleState(in *BattleStateInput) (*BattleStateSnapshot, error) {
	if in == nil {
		return nil, nil
	}
	for _, f := range []struct {
		name string
		v    *int
		max  int
	}{
		{"calc.battleState.attackerCurrentHp", in.AttackerCurrentHP, 0},
		{"calc.battleState.defenderCurrentHp", in.DefenderCurrentHP, 0},
		{"calc.battleState.hits", in.Hits, maxBattleStateHits},
	} {
		if f.v == nil {
			continue
		}
		if *f.v < 1 || (f.max > 0 && *f.v > f.max) {
			if f.max > 0 {
				return nil, Errorf(CodeInvalidInput, "%s は 1〜%d でなければならない", f.name, f.max)
			}
			return nil, Errorf(CodeInvalidInput, "%s は 1 以上でなければならない", f.name)
		}
	}
	if in.AttackerCurrentHP == nil && in.DefenderCurrentHP == nil && in.Hits == nil {
		return nil, nil
	}
	return &BattleStateSnapshot{AttackerCurrentHP: in.AttackerCurrentHP, DefenderCurrentHP: in.DefenderCurrentHP, Hits: in.Hits}, nil
}

func normalizeScreens(in *ScreensInput) ScreensSnapshot {
	if in == nil {
		return ScreensSnapshot{}
	}
	return ScreensSnapshot{
		Reflect: derefBool(in.Reflect), LightScreen: derefBool(in.LightScreen), AuroraVeil: derefBool(in.AuroraVeil),
	}
}

func derefBool(b *bool) bool { return b != nil && *b }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normalizeSP(in *SPInput) (SPSnapshot, error) {
	if in == nil || in.Hp == nil || in.Atk == nil || in.Def == nil || in.Spa == nil || in.Spd == nil || in.Spe == nil {
		return SPSnapshot{}, Errorf(CodeInvalidInput, "sp は6つの能力ポイントすべてが必要")
	}
	sp := SPSnapshot{Hp: *in.Hp, Atk: *in.Atk, Def: *in.Def, Spa: *in.Spa, Spd: *in.Spd, Spe: *in.Spe}
	total := 0
	for _, v := range []int{sp.Hp, sp.Atk, sp.Def, sp.Spa, sp.Spd, sp.Spe} {
		if v < 0 || v > engine.MaxSPPerStat {
			return SPSnapshot{}, Errorf(CodeInvalidInput, "sp は各 0〜%d でなければならない", engine.MaxSPPerStat)
		}
		total += v
	}
	if total > engine.MaxSPTotal {
		return SPSnapshot{}, Errorf(CodeInvalidInput, "sp の合計は%d以下でなければならない(%d)", engine.MaxSPTotal, total)
	}
	return sp, nil
}

func normalizeRanks(in *RankInput) (RankValues, error) {
	var r RankValues
	if in == nil {
		return r, nil
	}
	for _, f := range []struct {
		src *int
		dst *int
	}{{in.Atk, &r.Atk}, {in.Def, &r.Def}, {in.Spa, &r.Spa}, {in.Spd, &r.Spd}, {in.Spe, &r.Spe}} {
		if f.src == nil {
			continue
		}
		if *f.src < -maxRank || *f.src > maxRank {
			return r, Errorf(CodeInvalidInput, "ランクは -%d〜+%d でなければならない", maxRank, maxRank)
		}
		*f.dst = *f.src
	}
	return r, nil
}
