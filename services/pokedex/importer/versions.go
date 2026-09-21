package importer

// 取得元ごとの版・チェックサムと、投入が要るかどうかの判定(ADR-0101 §9)。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

// SourceVersion は1取得元の版とチェックサム(data_versions テーブルの行に対応)。
type SourceVersion struct {
	Source   string
	Version  string
	Checksum string
}

// sourcePattern / checksumPattern は data_versions の CHECK 制約と同じ形式(ADR-0015 §3)。
var (
	sourcePattern   = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	checksumPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Checksum はファイルのバイト列の sha256(16進)。
func Checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// validateVersions は incoming の形式を検証する(重複・source の形式・version 非空・
// checksum の形式・空でないこと)。
func validateVersions(vs []SourceVersion) error {
	if len(vs) == 0 {
		return fmt.Errorf("%w: 取得元の版が空", ErrInvalidInput)
	}
	seen := map[string]bool{}
	for _, v := range vs {
		if seen[v.Source] {
			return fmt.Errorf("%w: source が重複: %q", ErrInvalidInput, v.Source)
		}
		seen[v.Source] = true
		if !sourcePattern.MatchString(v.Source) {
			return fmt.Errorf("%w: source の形式が不正: %q", ErrInvalidInput, v.Source)
		}
		if v.Version == "" {
			return fmt.Errorf("%w: source %q の version が空", ErrInvalidInput, v.Source)
		}
		if !checksumPattern.MatchString(v.Checksum) {
			return fmt.Errorf("%w: source %q の checksum の形式が不正", ErrInvalidInput, v.Source)
		}
	}
	return nil
}

// NeedsImport は incoming を検証したうえで、投入が要るか(source の集合・各 version・
// 各 checksum のいずれかが applied と違うか)を返す。
func NeedsImport(applied, incoming []SourceVersion) (bool, error) {
	if err := validateVersions(incoming); err != nil {
		return false, err
	}
	if len(applied) != len(incoming) {
		return true, nil
	}
	byIncoming := map[string]SourceVersion{}
	for _, v := range incoming {
		byIncoming[v.Source] = v
	}
	for _, a := range applied {
		in, ok := byIncoming[a.Source]
		if !ok || in.Version != a.Version || in.Checksum != a.Checksum {
			return true, nil
		}
	}
	return false, nil
}
