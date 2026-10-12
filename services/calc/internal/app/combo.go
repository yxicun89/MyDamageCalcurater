package app

import (
	"context"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/domain"
)

// Combo は連続技の計算(POST /api/calc/combo。ADR-0253・ADR-0179 追記 CB-3)。
// 順番は 列挙(format・opponentSpeed・field・各個体の status・teraType)→ ID 解決(攻撃側 → 防御側 → 各手の技 → 相手の技)
// → 個体の検証 → メガの持ち物 → 締め切り → engine.CalcCombo。
// ランク変化の無い変化技の手・変化技の相手の技・多段の回数の上限と技との対応は engine の sentinel(invalid_input)に任せる。
// 計算イベントは発行しない(絶対ルール5。ADR-0253 §4)。
func (s *Service) Combo(ctx context.Context, req ComboRequest) (engine.ComboResult, error) {
	format, err := domain.ParseFormat(req.Format)
	if err != nil {
		return engine.ComboResult{}, err
	}
	oppSpeed, err := domain.ParseOpponentSpeed(req.OpponentSpeed)
	if err != nil {
		return engine.ComboResult{}, err
	}
	field, err := domain.ParseField(req.Field)
	if err != nil {
		return engine.ComboResult{}, err
	}
	attacker, err := s.ResolveIndividual("attacker", req.Attacker)
	if err != nil {
		return engine.ComboResult{}, err
	}
	defender, err := s.ResolveIndividual("defender", req.Defender)
	if err != nil {
		return engine.ComboResult{}, err
	}
	steps := make([]engine.ComboStep, 0, len(req.Steps))
	for _, st := range req.Steps {
		mv, err := s.ResolveMove(st.MoveID)
		if err != nil {
			return engine.ComboResult{}, err
		}
		steps = append(steps, engine.ComboStep{Move: mv, Critical: derefBool(st.Critical), Hits: derefInt(st.Hits)})
	}
	var oppMove *engine.Move
	if req.OpponentMoveID != nil {
		mv, err := s.ResolveMove(*req.OpponentMoveID)
		if err != nil {
			return engine.ComboResult{}, err
		}
		oppMove = &mv
	}
	if err := domain.ValidateIndividual("攻撃側", attacker); err != nil {
		return engine.ComboResult{}, err
	}
	if err := domain.ValidateIndividual("防御側", defender); err != nil {
		return engine.ComboResult{}, err
	}
	if err := domain.CheckMegaItem(s.master, "攻撃側", attacker.Species, attacker.Item); err != nil {
		return engine.ComboResult{}, err
	}
	if err := domain.CheckMegaItem(s.master, "防御側", defender.Species, defender.Item); err != nil {
		return engine.ComboResult{}, err
	}

	if s.expired(ctx) {
		return engine.ComboResult{}, ErrDeadlineExceeded
	}
	res, err := engine.CalcCombo(engine.ComboInput{
		Format: format, Attacker: attacker, Defender: defender, Field: field, TypeChart: s.master.TypeChart(),
		Steps: steps, OpponentMove: oppMove, OpponentSpeed: oppSpeed, SpeedField: req.SpeedField.engine(),
	})
	if err != nil {
		return engine.ComboResult{}, domain.FromEngine(err)
	}
	return res, nil
}
