package importer

// 技の機構の中身(move_mechanism_params。ADR-0142 §7)の変換。
//
// 機構(move_mechanisms。ADR-0121)のうち、取得元のフィールドの値だけで中身が決まるものの値を、
// 捨てずに1技1行で出す。技の名前では決めない。
//   - multi_hit: multihit(回数 or [最小, 最大])
//   - fixed_damage: damage("level" or 数値)。damageCallback 由来(残り HP の半分 等)は値が無いので中身なし
//   - ohko: ohko(true or タイプ名。タイプ名は相性表のタイプ ID に写す)
//   - alt_offense_stat / alt_defense_stat: override*(能力値・ポケモン)
//
// always_crit・ignore_defense_ranks は機構だけで決まるので中身を持たない。

import (
	"fmt"
	"sort"

	"example.com/pokecalc/services/internal/master"
)

// MoveMechanismParamsRow は move_mechanism_params の行(技1つ)。ゼロ値の項目は「その項目なし」。
type MoveMechanismParamsRow struct {
	MoveID           string
	MultiHitMin      int // 多段の最小回数。0 は多段の中身なし(Min == Max は固定回数)
	MultiHitMax      int
	FixedDamageLevel bool // 固定ダメージが攻撃側のレベルと同じ
	FixedDamageValue int  // 固定ダメージの数値。0 は数値なし
	OHKO             bool // 一撃必殺の中身あり
	OHKOImmuneType   string
	OffenseStat      string // 攻撃に使う能力値(atk / def / spa / spd / spe)
	OffensePokemon   string // attacker / defender
	DefenseStat      string // 防御に使う能力値
}

// masterRow は services/internal/master の行に写す(engine へ写像できるかの最終確認用)。
func (r MoveMechanismParamsRow) masterRow() *master.MoveMechanismParamsRow {
	return &master.MoveMechanismParamsRow{
		MultiHitMin: r.MultiHitMin, MultiHitMax: r.MultiHitMax,
		FixedDamageLevel: r.FixedDamageLevel, FixedDamageValue: r.FixedDamageValue,
		OHKO: r.OHKO, OHKOImmuneType: r.OHKOImmuneType,
		OffenseStat: r.OffenseStat, OffensePokemon: r.OffensePokemon, DefenseStat: r.DefenseStat,
	}
}

// moveParamStats は攻撃・防御に使える能力値(HP 以外)。
var moveParamStats = map[string]bool{"atk": true, "def": true, "spa": true, "spd": true, "spe": true}

// moveParamPokemon は取得元の overrideOffensivePokemon → engine の語彙。
var moveParamPokemon = map[string]string{"target": "defender", "source": "attacker"}

// moveMechanismParamsOf は1つの攻撃技の中身を返す。中身が無ければ nil。
// typeIDs は相性表のタイプ名(取得元の表記)→ タイプ ID。
func moveMechanismParamsOf(id string, sig ShowdownMoveMechanism, typeIDs map[string]string) (*MoveMechanismParamsRow, error) {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: 技 %q: %s", ErrInvalidData, id, fmt.Sprintf(format, args...))
	}
	row := MoveMechanismParamsRow{MoveID: id}
	var err error
	if row.MultiHitMin, row.MultiHitMax, err = parseMultihit(sig.Multihit); err != nil {
		return nil, bad("%v", err)
	}
	level, value, fixed, err := parseFixedDamage(sig.Damage)
	if err != nil {
		return nil, bad("%v", err)
	}
	if fixed {
		row.FixedDamageLevel, row.FixedDamageValue = level, value
	}
	immune, ohko, err := parseOHKO(sig.OHKO)
	if err != nil {
		return nil, bad("%v", err)
	}
	if ohko {
		row.OHKO = true
		if immune != "" {
			typeID, ok := typeIDs[immune]
			if !ok {
				return nil, bad("ohko のタイプ %q が相性表に無い", immune)
			}
			row.OHKOImmuneType = typeID
		}
	}
	if s := sig.OverrideOffensiveStat; s != "" {
		if !moveParamStats[s] {
			return nil, bad("overrideOffensiveStat が想定外: %q", s)
		}
		row.OffenseStat = s
	}
	if p := sig.OverrideOffensivePokemon; p != "" {
		pokemon, ok := moveParamPokemon[p]
		if !ok {
			return nil, bad("overrideOffensivePokemon が想定外: %q", p)
		}
		row.OffensePokemon = pokemon
	}
	if s := sig.OverrideDefensiveStat; s != "" {
		if !moveParamStats[s] {
			return nil, bad("overrideDefensiveStat が想定外: %q", s)
		}
		row.DefenseStat = s
	}
	if row == (MoveMechanismParamsRow{MoveID: id}) {
		return nil, nil
	}
	return &row, nil
}

// buildMoveMechanismParams は moves 表に入れる攻撃技の中身の行を技 ID の昇順で作る(変化技は持たない)。
func buildMoveMechanismParams(moves []ShowdownMove, rows []MoveRow, typeIDs map[string]string) ([]MoveMechanismParamsRow, error) {
	sdByID := make(map[string]ShowdownMove, len(moves))
	for _, m := range moves {
		sdByID[m.ID] = m
	}
	var out []MoveMechanismParamsRow
	for _, r := range rows {
		if r.Category == "status" {
			continue
		}
		sm, ok := sdByID[r.ID]
		if !ok || sm.Mechanism == nil {
			return nil, fmt.Errorf("%w: 技 %q の機構の判定材料(mechanism)が無い(tools/importer で取り直す)", ErrInvalidData, r.ID)
		}
		row, err := moveMechanismParamsOf(r.ID, *sm.Mechanism, typeIDs)
		if err != nil {
			return nil, err
		}
		if row != nil {
			out = append(out, *row)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MoveID < out[j].MoveID })
	return out, nil
}
