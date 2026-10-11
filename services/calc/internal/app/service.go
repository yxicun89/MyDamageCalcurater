package app

import (
	"context"
	"time"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/calc/internal/domain"
)

// Service は計算・一括・逆算のユースケース。1 リクエストごとに作っても、使い回してもよい(状態を持たない)。
type Service struct {
	master    Master
	publisher EventPublisher
	expired   Expired
	now       func() time.Time
}

// New は Service を作る。publisher が nil なら発行しない。expired が nil なら ctx.Err() だけを見る。
func New(master Master, publisher EventPublisher, expired Expired) *Service {
	if publisher == nil {
		publisher = noopPublisher{}
	}
	if expired == nil {
		expired = func(ctx context.Context) bool { return ctx.Err() != nil }
	}
	return &Service{master: master, publisher: publisher, expired: expired, now: time.Now}
}

// Calc は 1 回のダメージ計算(POST /api/calc)。成功したら Detail 入りのイベントを発行する(ADR-0212 §7)。
func (s *Service) Calc(ctx context.Context, meta Meta, req CalcRequest) (engine.DamageResult, error) {
	format, err := domain.ParseFormat(req.Format)
	if err != nil {
		return engine.DamageResult{}, err
	}
	attacker, err := s.ResolveIndividual("attacker", req.Attacker)
	if err != nil {
		return engine.DamageResult{}, err
	}
	defender, err := s.ResolveIndividual("defender", req.Defender)
	if err != nil {
		return engine.DamageResult{}, err
	}
	move, err := s.ResolveMove(req.MoveID)
	if err != nil {
		return engine.DamageResult{}, err
	}
	field, err := domain.ParseField(req.Field)
	if err != nil {
		return engine.DamageResult{}, err
	}
	if err := domain.ValidateIndividual("攻撃側", attacker); err != nil {
		return engine.DamageResult{}, err
	}
	if err := domain.ValidateIndividual("防御側", defender); err != nil {
		return engine.DamageResult{}, err
	}
	if err := domain.CheckMegaItem(s.master, "攻撃側", attacker.Species, attacker.Item); err != nil {
		return engine.DamageResult{}, err
	}
	if err := domain.CheckMegaItem(s.master, "防御側", defender.Species, defender.Item); err != nil {
		return engine.DamageResult{}, err
	}
	state, err := domain.BattleStateFrom(req.BattleState)
	if err != nil {
		return engine.DamageResult{}, err
	}

	if s.expired(ctx) {
		return engine.DamageResult{}, ErrDeadlineExceeded
	}
	res, err := engine.CalcDamage(engine.DamageInput{
		Format: format, Attacker: attacker, Defender: defender, Move: move, Field: field,
		Critical: req.Options.critical(), TypeChart: s.master.TypeChart(), State: state,
	})
	if err != nil {
		return engine.DamageResult{}, domain.FromEngine(err)
	}
	// イベント発行は非同期・応答をブロックしない(CLAUDE.md 絶対ルール5・ADR-0212 §6)。
	s.publisher.Publish(Event{Meta: meta, Operation: OperationCalc, OccurredAt: s.now().UTC(), Calc: &CalcEvent{Request: req, Result: res}})
	return res, nil
}

// Bulk は防御側の努力値の型ごとの一括計算(POST /api/calc/bulk)。
func (s *Service) Bulk(ctx context.Context, meta Meta, req BulkRequest) (BulkOutput, error) {
	format, err := domain.ParseFormat(req.Format)
	if err != nil {
		return BulkOutput{}, err
	}
	attacker, err := s.ResolveIndividual("attacker", req.Attacker)
	if err != nil {
		return BulkOutput{}, err
	}
	species, err := s.resolveSpecies("defenderSpeciesKey", req.DefenderSpeciesKey)
	if err != nil {
		return BulkOutput{}, err
	}
	move, err := s.ResolveMove(req.MoveID)
	if err != nil {
		return BulkOutput{}, err
	}
	field, err := domain.ParseField(req.Field)
	if err != nil {
		return BulkOutput{}, err
	}
	variants, err := s.resolveItems("itemVariants", req.ItemVariants)
	if err != nil {
		return BulkOutput{}, err
	}
	if err := domain.ValidateIndividual("攻撃側", attacker); err != nil {
		return BulkOutput{}, err
	}
	if err := domain.ValidateIndividual("防御側の種族", engine.Individual{Species: species}); err != nil {
		return BulkOutput{}, err
	}
	if err := domain.CheckMegaItem(s.master, "攻撃側", attacker.Species, attacker.Item); err != nil {
		return BulkOutput{}, err
	}
	if err := domain.CheckMegaItems(s.master, "itemVariants", species, variants); err != nil {
		return BulkOutput{}, err
	}
	var defenderAbilityOverride *string
	var defenderOverride engine.DefenderOverride
	if ov := req.DefenderOverride; ov != nil {
		defenderAbilityOverride = ov.AbilityID
		status, err := domain.ParseStatus("defenderOverride", ov.Status)
		if err != nil {
			return BulkOutput{}, err
		}
		defenderOverride = engine.DefenderOverride{Ranks: ov.Ranks.engine(), Status: status}
	}
	defenderAbilities, err := s.resolveAbilityCandidates("defenderOverride.abilityId", species, defenderAbilityOverride)
	if err != nil {
		return BulkOutput{}, err
	}

	if s.expired(ctx) {
		return BulkOutput{}, ErrDeadlineExceeded
	}
	res, err := engine.CalcBulk(engine.BulkInput{
		Format: format, Attacker: attacker, DefenderSpecies: species, Move: move, Field: field,
		Critical: req.Options.critical(), PresetKeys: req.Presets, ItemVariants: variants,
		DefenderAbilities: defenderAbilities, DefenderOverride: defenderOverride, TypeChart: s.master.TypeChart(),
	})
	if err != nil {
		return BulkOutput{}, domain.FromEngine(err)
	}
	// envelope だけを発行する(Detail は付けない。比較検討中の候補で「よく使う相手」の集計を
	// 汚染しないため。ADR-0212 §7.1)。
	s.publisher.Publish(Event{Meta: meta, Operation: OperationBulk, OccurredAt: s.now().UTC()})
	natureIDs := make([]*string, 0, len(res.Rows))
	for _, row := range res.Rows {
		natureIDs = append(natureIDs, s.natureID(row.Defender.Nature))
	}
	return BulkOutput{Result: res, NatureIDs: natureIDs}, nil
}

// Reverse は観測からの逆算(POST /api/calc/reverse)。
func (s *Service) Reverse(ctx context.Context, meta Meta, req ReverseRequest) (ReverseOutput, error) {
	format, err := domain.ParseFormat(req.Format)
	if err != nil {
		return ReverseOutput{}, err
	}
	known, err := s.ResolveIndividual("known", req.Known)
	if err != nil {
		return ReverseOutput{}, err
	}
	species, err := s.resolveSpecies("unknownSpeciesKey", req.UnknownSpeciesKey)
	if err != nil {
		return ReverseOutput{}, err
	}
	move, err := s.ResolveMove(req.MoveID)
	if err != nil {
		return ReverseOutput{}, err
	}
	field, err := domain.ParseField(req.Field)
	if err != nil {
		return ReverseOutput{}, err
	}
	items, err := s.resolveItems("itemCandidates", req.ItemCandidates)
	if err != nil {
		return ReverseOutput{}, err
	}
	observations, err := domain.ConvertObservations(req.Observations)
	if err != nil {
		return ReverseOutput{}, err
	}
	if err := domain.ValidateIndividual("既知の側", known); err != nil {
		return ReverseOutput{}, err
	}
	if err := domain.ValidateIndividual("推定側の種族", engine.Individual{Species: species}); err != nil {
		return ReverseOutput{}, err
	}
	if err := domain.CheckMegaItem(s.master, "既知の側", known.Species, known.Item); err != nil {
		return ReverseOutput{}, err
	}
	if err := domain.CheckMegaItems(s.master, "itemCandidates", species, items); err != nil {
		return ReverseOutput{}, err
	}
	unknownAbilities, err := s.resolveAbilityCandidates("unknownAbilityId", species, req.UnknownAbilityID)
	if err != nil {
		return ReverseOutput{}, err
	}

	if s.expired(ctx) {
		return ReverseOutput{}, ErrDeadlineExceeded
	}
	res, err := engine.CalcReverse(engine.ReverseInput{
		Format: format, Side: req.Side, Known: known, UnknownSpecies: species,
		Move: move, Field: field, Critical: req.Options.critical(), ItemCandidates: items,
		Observations: observations, MaxCandidates: derefInt(req.MaxCandidates), UnknownAbilities: unknownAbilities,
		TypeChart: s.master.TypeChart(),
	})
	if err != nil {
		return ReverseOutput{}, domain.FromEngine(err)
	}
	// envelope だけを発行する(総当たり探索であり calc と同じ形の Detail に当てはまらない。ADR-0212 §7.1)。
	s.publisher.Publish(Event{Meta: meta, Operation: OperationReverse, OccurredAt: s.now().UTC()})
	natureIDs := make([]*string, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		natureIDs = append(natureIDs, s.natureID(c.Nature))
	}
	return ReverseOutput{Result: res, NatureIDs: natureIDs}, nil
}
