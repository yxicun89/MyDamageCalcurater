package importer

// 技の追加効果(命中時のランク変化)の変換(ADR-0107 決定6)。
//
// 取得元は Showdown だけ(@smogon/calc は真偽値しか持たず突き合わせできない。ADR-0107「調査」)。
// Showdown の表現は3通り(ADR-0107「調査」):
//  1. トップレベルの self.boosts             → 命中すれば必ず発動(確率100に正規化)
//  2. secondary.self.boosts                  → 確率つきの自分のランク変化
//  3. secondary.boosts(self を伴わない)      → 確率つきの相手のランク変化
//
// ピン留めした取得元にはこれらが2つ以上重なる技が無い(ADR-0107 決定3)。重なりが見つかったら
// blocker にして人に知らせる(黙って片方を落とさない)。

import (
	"sort"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// rankEntry は Showdown の1つのランク変化表現を engine.MoveEffect に正規化する前の中間形。
type rankEntry struct {
	chance int
	target engine.RankTarget
	boosts map[string]int
}

// moveEffectEntries は1つの技が持つランク変化エントリを集める(0件のことも複数件のこともある)。
func moveEffectEntries(m ShowdownMove) []rankEntry {
	var entries []rankEntry
	if m.Self != nil && len(m.Self.Boosts) > 0 {
		entries = append(entries, rankEntry{chance: 100, target: engine.RankTargetSelf, boosts: m.Self.Boosts})
	}
	if m.Secondary != nil {
		if m.Secondary.Self != nil && len(m.Secondary.Self.Boosts) > 0 {
			entries = append(entries, rankEntry{chance: m.Secondary.Chance, target: engine.RankTargetSelf, boosts: m.Secondary.Self.Boosts})
		}
		if len(m.Secondary.Boosts) > 0 {
			entries = append(entries, rankEntry{chance: m.Secondary.Chance, target: engine.RankTargetOpponent, boosts: m.Secondary.Boosts})
		}
	}
	return entries
}

// secondaryBoostCount は取得元の secondaries 配列のうち、ランク変化(boosts)を伴う要素の件数を数える。
// 状態異常・ひるみ等(boosts を持たない要素)は決定3・6 の対象外なので数えない(ADR-0107 2026-09-23 追記)。
func secondaryBoostCount(secondaries []ShowdownSecondary) int {
	n := 0
	for _, s := range secondaries {
		if len(s.Boosts) > 0 || (s.Self != nil && len(s.Self.Boosts) > 0) {
			n++
		}
	}
	return n
}

// buildMoveEffects は Showdown のスナップショットから move_effects の行を作る(ADR-0107 決定6)。
// moves 表に採らなかった技(included に無い技)は対象にしない(規則1)。
func buildMoveEffects(moves []ShowdownMove, included map[string]bool) ([]EffectRow, []Finding, []Finding, error) {
	sorted := append([]ShowdownMove(nil), moves...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	var rows []EffectRow
	var warnings, blockers []Finding

	for _, m := range sorted {
		if !included[m.ID] {
			continue
		}
		// secondaries(配列)のうちランク変化(boosts)を伴う要素が2件以上あるのは、
		// 取得元が単一の secondary では表現しきれない技(ADR-0107 決定3 の前提が崩れている)。
		if secondaryBoostCount(m.Secondaries) >= 2 {
			blockers = append(blockers, Finding{Kind: KindMoveEffectAmbiguous, ID: m.ID})
			continue
		}
		entries := moveEffectEntries(m)
		if len(entries) == 0 {
			continue
		}
		if len(entries) >= 2 {
			blockers = append(blockers, Finding{Kind: KindMoveEffectAmbiguous, ID: m.ID})
			continue
		}
		e := entries[0]

		// accuracy/evasion は engine の Ranks に持ち場が無いので落とす(規則4)。atk 等の他のキーと
		// 混ざっていても、落としたこと自体は必ず警告に残す(黙って落とさない。2026-09-23 追記)。
		// それ以外の未知のキー(hp 等。実測に無い形)は master.EncodeMoveEffect の検証に委ねる。
		// そのエラーは buildMoveEffects の戻り値の err として Convert 全体を中断させる
		// (既存の validateOutputMapsToEngine と同じ「engine に写せないデータは止める」流儀)。
		stages := map[engine.StatKey]int{}
		hasUnsupportedStat := false
		for k, v := range e.boosts {
			if k == "accuracy" || k == "evasion" {
				hasUnsupportedStat = true
				continue
			}
			stages[engine.StatKey(k)] = v
		}
		if hasUnsupportedStat {
			warnings = append(warnings, Finding{Kind: KindMoveEffectUnsupportedStat, ID: m.ID})
		}
		if len(stages) == 0 {
			continue
		}

		canon, err := master.EncodeMoveEffect(engine.MoveEffect{Chance: e.chance, Target: e.target, Stages: stages})
		if err != nil {
			return nil, nil, nil, err
		}
		rows = append(rows, EffectRow{ID: m.ID, Effect: canon})
	}
	return rows, warnings, blockers, nil
}
