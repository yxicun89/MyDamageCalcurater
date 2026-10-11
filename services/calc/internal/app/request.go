package app

// 操作の入力(adapters/httpapi が生成型から写す)。省略の有無(nil)は契約のまま保つ
// (計算イベントにはリクエストのままを載せるため。ADR-0212 §7)。

import (
	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/domain"
)

// Individual は個体の指定(ID は未解決のまま)。
type Individual struct {
	SpeciesKey string
	NatureID   string
	Level      *int
	AbilityID  *string
	ItemID     *string
	SP         engine.Stats
	Ranks      *Ranks
	Status     *domain.Enum
	TeraType   *domain.Enum
}

// Ranks はランク補正の指定(省略した項目は nil = 0)。
type Ranks struct {
	Atk, Def, SpA, SpD, Spe *int
}

func (r *Ranks) engine() engine.Ranks {
	if r == nil {
		return engine.Ranks{}
	}
	return engine.Ranks{Atk: derefInt(r.Atk), Def: derefInt(r.Def), SpA: derefInt(r.SpA), SpD: derefInt(r.SpD), Spe: derefInt(r.Spe)}
}

// Options は計算の任意指定。
type Options struct {
	Critical *bool
}

func (o *Options) critical() bool { return o != nil && o.Critical != nil && *o.Critical }

// CalcRequest は計算(POST /api/calc)の入力。
type CalcRequest struct {
	Format      domain.Enum
	Attacker    Individual
	Defender    Individual
	MoveID      string
	Field       *domain.Field
	Options     *Options
	BattleState *domain.BattleState
}

// DefenderOverride は一括計算の防御側の上書き(ADR-0216・ADR-0214)。
type DefenderOverride struct {
	AbilityID *string
	Status    *domain.Enum
	Ranks     *Ranks
}

// BulkRequest は一括計算(POST /api/calc/bulk)の入力。
// Presets は契約の列挙値をそのまま写したもの(未知の値の検証は engine の sentinel に任せる。WASM 境界と同じ失敗にするため)。
// ItemVariants の nil は省略(engine が「持ち物なしの1通り」に既定する)、要素の nil は持ち物なし。
type BulkRequest struct {
	Format             domain.Enum
	Attacker           Individual
	DefenderSpeciesKey string
	MoveID             string
	Field              *domain.Field
	Options            *Options
	Presets            []engine.PresetKey
	ItemVariants       *[]*string
	DefenderOverride   *DefenderOverride
}

// ReverseRequest は逆算(POST /api/calc/reverse)の入力。Side は engine の sentinel で検証する(ADR-0200 §4)。
type ReverseRequest struct {
	Format            domain.Enum
	Side              engine.ReverseSide
	Known             Individual
	UnknownSpeciesKey string
	MoveID            string
	Field             *domain.Field
	Options           *Options
	ItemCandidates    *[]*string
	Observations      []domain.Observation
	MaxCandidates     *int
	UnknownAbilityID  *string
}

// BulkOutput は一括計算の結果。NatureIDs は Result.Rows と同じ順の性格 ID(引けなければ nil)。
type BulkOutput struct {
	Result    engine.BulkResult
	NatureIDs []*string
}

// ReverseOutput は逆算の結果。NatureIDs は Result.Candidates と同じ順の性格 ID(引けなければ nil)。
type ReverseOutput struct {
	Result    engine.ReverseResult
	NatureIDs []*string
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
