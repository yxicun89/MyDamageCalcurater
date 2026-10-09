package importer

// 技のフラグ(move_flags。ADR-0178)の導出と照合。
//
// 語彙(master.AllMoveFlags)のうち、bite・bullet・contact・pulse・punch・slicing・sound は取得元の flags の同名のキー、
// recoil は反動(recoil があるか hasCrashDamage が真)、secondary は追加効果(secondary があるか secondaries が空でない)。
// 語彙に無い取得元のフラグ(protect・mirror 等)はダメージに効かないので捨てる。

import (
	"bytes"
	"fmt"
	"slices"
	"sort"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

// hasRecoil は取得元の recoil(json の [分子, 分母] か null)が反動ありかを返す(キーが無い・null は無し)。
func hasRecoil(raw []byte) bool {
	r := bytes.TrimSpace(raw)
	return len(r) > 0 && !bytes.Equal(r, []byte("null"))
}

// deriveMoveFlags は取得元の値から語彙のフラグを昇順・重複なしで導く。
func deriveMoveFlags(sourceFlags []string, recoil, secondary bool) []string {
	var out []string
	for _, f := range sourceFlags {
		if master.IsMoveFlag(f) && f != string(engine.MoveFlagRecoil) && f != string(engine.MoveFlagSecondary) {
			out = append(out, f)
		}
	}
	if recoil {
		out = append(out, string(engine.MoveFlagRecoil))
	}
	if secondary {
		out = append(out, string(engine.MoveFlagSecondary))
	}
	sort.Strings(out)
	return slices.Compact(out)
}

// showdownMoveFlags は Showdown の技のフラグを導く。flags が無い(nil)なら ErrInvalidData
// (黙ってフラグなしにしない。取得物のデコードは同じ条件を ErrInvalidInput にする)。
func showdownMoveFlags(m ShowdownMove) ([]string, error) {
	if m.Flags == nil {
		return nil, fmt.Errorf("%w: 技 %q に flags が無い(ADR-0178)", ErrInvalidData, m.ID)
	}
	secondary := m.Secondary != nil || len(m.Secondaries) > 0
	return deriveMoveFlags(*m.Flags, hasRecoil(m.Recoil) || m.HasCrashDamage, secondary), nil
}

// calcMoveFlags は calc の技のフラグを Showdown と同じ規則で導く(照合用)。
func calcMoveFlags(id string, m CalcMove) ([]string, error) {
	if m.Flags == nil {
		return nil, fmt.Errorf("%w: calc の技 %q に flags が無い(ADR-0178)", ErrInvalidData, id)
	}
	return deriveMoveFlags(*m.Flags, hasRecoil(m.Recoil) || m.HasCrashDamage, m.Secondaries), nil
}

// moveFlagRows は技ごとのフラグを (技 ID, フラグ) の昇順の行にする。フラグの無い技は行を作らない。
func moveFlagRows(byMove map[string][]string) []MoveFlagRow {
	var rows []MoveFlagRow
	for _, id := range sortedKeysRaw(byMove) {
		for _, f := range byMove[id] {
			rows = append(rows, MoveFlagRow{MoveID: id, Flag: f})
		}
	}
	return rows
}

// computeMoveFlagSummary は moves 表の攻撃技のうちフラグを持つ技の数と、フラグごとの攻撃技の数を数える(ADR-0178 §2)。
func computeMoveFlagSummary(out Output) MoveFlagSummary {
	s := MoveFlagSummary{ByFlag: map[string]int{}}
	attack := map[string]bool{}
	for _, m := range out.Moves {
		if m.Category != "status" {
			s.Attack++
			attack[m.ID] = true
		}
	}
	withFlag := map[string]bool{}
	for _, r := range out.MoveFlags {
		if !attack[r.MoveID] {
			continue
		}
		withFlag[r.MoveID] = true
		s.ByFlag[r.Flag]++
	}
	s.WithFlag = len(withFlag)
	return s
}
