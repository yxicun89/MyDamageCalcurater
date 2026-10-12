package domain

// 公開の検索 API の入力の検証と LIKE のパターン(ADR-0105 §3)。どれも DB を呼ぶ前に行う。

import (
	"regexp"
	"strings"

	"example.com/pokecalc/services/internal/master"
)

const (
	DefaultSearchLimit = 50
	MinSearchLimit     = 1
	MaxSearchLimit     = 200

	// MaxSearchOffset は learners の offset の上限(契約の maximum。int32 に収まる安全側の値)。
	MaxSearchOffset = 10000

	// MaxBatchIDsCount は getMovesByIds の ids の件数上限(契約の maxItems と同じ。ADR-0208 の
	// 前例どおり、生成ラッパは配列のスキーマを検証しないため自前に検査する)。
	MaxBatchIDsCount = 64
)

// Enum は契約の列挙の値と、それが契約に含まれるか(Known)。含まれるかの判定は adapters が生成型で行い、
// 列挙の値の一覧をここに複製しない(契約が正。ADR-0252 と同じ)。
type Enum struct {
	Value string
	Known bool
}

// speciesKeyPattern は getSpecies の path パラメータの形式({図鑑番号4桁}-{フォルム3桁})。
var speciesKeyPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{3}$`)

// likeEscaper は LIKE の特殊文字(\ % _)を \ でエスケープする(1回の走査。エスケープの責任はここ1か所)。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// LikePattern は q を前方一致のパターンにする(省略・空は全件 "%")。
func LikePattern(q string) string {
	return likeEscaper.Replace(q) + "%"
}

// SpeciesSearchPatterns は種族検索の LIKE のパターンの組を返す(ADR-0324)。
// pattern は q の前方一致、megaPattern は「メガ + q」の前方一致(メガ種族だけに使う)。
// 後者で、メガを除いた基本種名(ルカリオ)でもメガ種族(メガルカリオ)が当たる。
func SpeciesSearchPatterns(q string) (pattern, megaPattern string) {
	return LikePattern(q), likeEscaper.Replace(master.MegaNamePrefix) + LikePattern(q)
}

// ResolveLimit は limit の既定値・範囲(1〜200)を検証する。
func ResolveLimit(p *int) (int32, error) {
	if p == nil {
		return DefaultSearchLimit, nil
	}
	if *p < MinSearchLimit || *p > MaxSearchLimit {
		return 0, Errorf(CodeInvalidInput, "limit は %d〜%d でなければならない: %d", MinSearchLimit, MaxSearchLimit, *p)
	}
	return int32(*p), nil
}

// ResolveOffset は offset の既定値(0)・範囲(0〜10000)を検証する。
func ResolveOffset(p *int) (int32, error) {
	if p == nil {
		return 0, nil
	}
	if *p < 0 || *p > MaxSearchOffset {
		return 0, Errorf(CodeInvalidInput, "offset は 0〜%d でなければならない: %d", MaxSearchOffset, *p)
	}
	return int32(*p), nil
}

// CheckFormat は format の値を検証する(省略可。v1 では結果に影響しない。ADR-0105 §3)。
func CheckFormat(p *Enum) error {
	if p == nil || p.Known {
		return nil
	}
	return Errorf(CodeInvalidEnum, "format が不正: %q", p.Value)
}

// CheckSpeciesKey は種族キーの形式を検証する。
func CheckSpeciesKey(key string) error {
	if !speciesKeyPattern.MatchString(key) {
		return Errorf(CodeInvalidInput, "種族キーの形式が不正: %q", key)
	}
	return nil
}

// CheckBatchIDs は getMovesByIds の ids の件数(1〜64)を検証する。
func CheckBatchIDs(ids []string) error {
	if len(ids) == 0 {
		return Errorf(CodeInvalidInput, "ids が空: 1〜%d 件でなければならない", MaxBatchIDsCount)
	}
	if len(ids) > MaxBatchIDsCount {
		return Errorf(CodeInvalidInput, "ids は %d 件以下でなければならない: %d 件", MaxBatchIDsCount, len(ids))
	}
	return nil
}
