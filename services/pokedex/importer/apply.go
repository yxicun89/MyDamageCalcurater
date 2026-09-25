package importer

// pokedex の DB への冪等な投入(ADR-0101 §9)。SQL は sqlc(services/pokedex/db/query/)で
// 生成した internal/store 経由でだけ発行する(手書きの SQL 文字列は持たない)。

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"example.com/pokecalc/services/pokedex/internal/store"
)

// AppliedVersions は data_versions の現在の内容を返す(source 順)。
func AppliedVersions(ctx context.Context, db *sql.DB) ([]SourceVersion, error) {
	rows, err := store.New(db).ListDataVersions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SourceVersion, 0, len(rows))
	for _, r := range rows {
		out = append(out, SourceVersion{Source: r.Source, Version: r.Version, Checksum: r.Checksum})
	}
	return out, nil
}

func strToNull(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

func accuracyToNull(v int) sql.NullInt16 {
	if v == 0 {
		return sql.NullInt16{}
	}
	return sql.NullInt16{Int16: int16(v), Valid: true}
}

func dateToNull(s string) (sql.NullTime, error) {
	if s == "" {
		return sql.NullTime{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return sql.NullTime{}, fmt.Errorf("%w: 日付の形式が不正: %q", ErrInvalidData, s)
	}
	return sql.NullTime{Time: t, Valid: true}, nil
}

// Apply は Output を1トランザクションで pokedex の DB に全置き換えする。
// 既存の showdown_id に対する species の key が変わる投入は ErrKeyChanged で止め、DB を変えない。
// 自己参照の外部キー(species.base_species_key)があるので、削除はメガを先に、挿入はメガを後にする。
func Apply(ctx context.Context, db *sql.DB, out Output, versions []SourceVersion, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() //nolint:errcheck // Commit 後は sql.ErrTxDone になるだけ

	q := store.New(tx)

	existing, err := q.ListSpeciesKeys(ctx)
	if err != nil {
		return err
	}
	existingKeyByShowdownID := make(map[string]string, len(existing))
	existingShowdownIDByKey := make(map[string]string, len(existing))
	for _, e := range existing {
		existingKeyByShowdownID[e.ShowdownID] = e.Key
		existingShowdownIDByKey[e.Key] = e.ShowdownID
	}
	for _, sp := range out.Species {
		if oldKey, ok := existingKeyByShowdownID[sp.ShowdownID]; ok && oldKey != sp.Key {
			return fmt.Errorf("%w: showdown_id %q の key が %q → %q", ErrKeyChanged, sp.ShowdownID, oldKey, sp.Key)
		}
		// 逆向き: 以前その key を持っていたのが別の showdown_id なら、key を乗っ取る投入も止める
		// (team-svc 等が保存した key の指す種族が入れ替わるのを防ぐ)。
		if oldShowdownID, ok := existingShowdownIDByKey[sp.Key]; ok && oldShowdownID != sp.ShowdownID {
			return fmt.Errorf("%w: key %q の showdown_id が %q → %q", ErrKeyChanged, sp.Key, oldShowdownID, sp.ShowdownID)
		}
	}

	deletes := []func(context.Context) error{
		q.DeleteRegulationSpecies, q.DeleteRegulationMoves, q.DeleteRegulationItems, q.DeleteRegulationAbilities,
		q.DeleteRegulations,
		q.DeleteLearnsets,
		q.DeleteSpeciesAbilities,
		q.DeleteItemEffects, q.DeleteAbilityEffects, q.DeleteMoveEffects, q.DeleteMoveMechanisms,
		q.DeleteMegaSpecies, q.DeleteRemainingSpecies,
		q.DeleteMoves, q.DeleteItems, q.DeleteAbilities,
		q.DeleteTypeChart, q.DeleteTypes,
		q.DeleteNatures,
		q.DeleteDataVersions,
	}
	for _, del := range deletes {
		if err := del(ctx); err != nil {
			return err
		}
	}

	for _, t := range out.Types {
		if err := q.InsertType(ctx, store.InsertTypeParams{ID: t.ID, SortOrder: uint16(t.SortOrder), NameJa: t.NameJa, NameJaSource: t.NameJaSource}); err != nil {
			return err
		}
	}
	for _, c := range out.TypeChart {
		if err := q.InsertTypeChart(ctx, store.InsertTypeChartParams{AttackType: c.AttackType, DefenseType: c.DefenseType, Code: uint8(c.Code)}); err != nil {
			return err
		}
	}
	for _, a := range out.Abilities {
		if err := q.InsertAbility(ctx, store.InsertAbilityParams{ID: a.ID, NameJa: a.NameJa, NameJaSource: a.NameJaSource, NameEn: a.NameEn}); err != nil {
			return err
		}
	}
	for _, it := range out.Items {
		if err := q.InsertItem(ctx, store.InsertItemParams{ID: it.ID, NameJa: it.NameJa, NameJaSource: it.NameJaSource, NameEn: it.NameEn}); err != nil {
			return err
		}
	}
	for _, m := range out.Moves {
		if err := q.InsertMove(ctx, store.InsertMoveParams{
			ID: m.ID, NameJa: m.NameJa, NameJaSource: m.NameJaSource, NameEn: m.NameEn,
			Type: m.Type, Category: m.Category, Power: uint16(m.Power),
			Accuracy: accuracyToNull(m.Accuracy), Pp: uint8(m.PP), Priority: int8(m.Priority),
		}); err != nil {
			return err
		}
	}

	// species: 自己参照の外部キー(base_species_key)があるので非メガを先に、メガを後に挿入する。
	ordered := make([]SpeciesRow, 0, len(out.Species))
	for _, sp := range out.Species {
		if !sp.IsMega {
			ordered = append(ordered, sp)
		}
	}
	for _, sp := range out.Species {
		if sp.IsMega {
			ordered = append(ordered, sp)
		}
	}
	for _, sp := range ordered {
		if err := q.InsertSpecies(ctx, store.InsertSpeciesParams{
			Key: sp.Key, DexNo: uint16(sp.DexNo), Form: uint16(sp.Form), ShowdownID: sp.ShowdownID,
			NameJa: sp.NameJa, NameJaSource: sp.NameJaSource, NameEn: sp.NameEn,
			Type1: sp.Type1, Type2: strToNull(sp.Type2),
			BaseHp: uint16(sp.BaseHP), BaseAtk: uint16(sp.BaseAtk), BaseDef: uint16(sp.BaseDef),
			BaseSpa: uint16(sp.BaseSpA), BaseSpd: uint16(sp.BaseSpD), BaseSpe: uint16(sp.BaseSpe),
			IsMega: sp.IsMega, BaseSpeciesKey: strToNull(sp.BaseSpeciesKey), RequiredItemID: strToNull(sp.RequiredItemID),
		}); err != nil {
			return err
		}
	}
	for _, sp := range out.Species {
		for _, a := range sp.Abilities {
			if err := q.InsertSpeciesAbility(ctx, store.InsertSpeciesAbilityParams{SpeciesKey: sp.Key, Slot: uint8(a.Slot), AbilityID: a.AbilityID}); err != nil {
				return err
			}
		}
	}
	for _, e := range out.ItemEffects {
		if err := q.InsertItemEffect(ctx, store.InsertItemEffectParams{ItemID: e.ID, Effect: json.RawMessage(e.Effect)}); err != nil {
			return err
		}
	}
	for _, e := range out.AbilityEffects {
		if err := q.InsertAbilityEffect(ctx, store.InsertAbilityEffectParams{AbilityID: e.ID, Effect: json.RawMessage(e.Effect)}); err != nil {
			return err
		}
	}
	for _, e := range out.MoveEffects {
		if err := q.InsertMoveEffect(ctx, store.InsertMoveEffectParams{MoveID: e.ID, Effect: json.RawMessage(e.Effect)}); err != nil {
			return err
		}
	}
	for _, m := range out.MoveMechanisms {
		if err := q.InsertMoveMechanism(ctx, store.InsertMoveMechanismParams{MoveID: m.MoveID, Mechanism: m.Mechanism}); err != nil {
			return err
		}
	}
	for _, l := range out.Learnsets {
		if err := q.InsertLearnset(ctx, store.InsertLearnsetParams{SpeciesKey: l.SpeciesKey, MoveID: l.MoveID}); err != nil {
			return err
		}
	}
	for _, r := range out.Regulations {
		startsOn, err := dateToNull(r.StartsOn)
		if err != nil {
			return err
		}
		endsOn, err := dateToNull(r.EndsOn)
		if err != nil {
			return err
		}
		if err := q.InsertRegulation(ctx, store.InsertRegulationParams{ID: r.ID, NameJa: r.NameJa, IsDefault: r.IsDefault, StartsOn: startsOn, EndsOn: endsOn}); err != nil {
			return err
		}
	}
	for _, m := range out.RegulationSpecies {
		if err := q.InsertRegulationSpecies(ctx, store.InsertRegulationSpeciesParams{RegulationID: m.RegulationID, SpeciesKey: m.MemberID}); err != nil {
			return err
		}
	}
	for _, m := range out.RegulationMoves {
		if err := q.InsertRegulationMove(ctx, store.InsertRegulationMoveParams{RegulationID: m.RegulationID, MoveID: m.MemberID}); err != nil {
			return err
		}
	}
	for _, m := range out.RegulationItems {
		if err := q.InsertRegulationItem(ctx, store.InsertRegulationItemParams{RegulationID: m.RegulationID, ItemID: m.MemberID}); err != nil {
			return err
		}
	}
	for _, m := range out.RegulationAbilities {
		if err := q.InsertRegulationAbility(ctx, store.InsertRegulationAbilityParams{RegulationID: m.RegulationID, AbilityID: m.MemberID}); err != nil {
			return err
		}
	}
	for _, n := range out.Natures {
		if err := q.InsertNature(ctx, store.InsertNatureParams{
			ID: n.ID, NameJa: n.NameJa, NameJaSource: n.NameJaSource, NameEn: n.NameEn,
			Plus: strToNull(n.Plus), Minus: strToNull(n.Minus),
		}); err != nil {
			return err
		}
	}
	for _, v := range versions {
		if err := q.InsertDataVersion(ctx, store.InsertDataVersionParams{Source: v.Source, Version: v.Version, Checksum: v.Checksum, ImportedAt: now}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Run は RunStore(ctx, NewSQLStore(db), ...) と同じ挙動(AppliedVersions → NeedsImport
// (force なら常に投入)→ Apply の順)。取り込んだら true を返す(ADR-0104 §10)。
func Run(ctx context.Context, db *sql.DB, out Output, versions []SourceVersion, now time.Time, force bool) (bool, error) {
	return RunStore(ctx, NewSQLStore(db), out, versions, now, force)
}
