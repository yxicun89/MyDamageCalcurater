package httpapi

// 候補・観測の件数上限の実行時検証(ADR-0208。issue #110)。
//
// 契約(api/openapi.yaml)には maxItems / uniqueItems / maximum を書いたが、生成ラッパ
// (api.ServerInterfaceWrapper)は本文のスキーマを検証しない(ヘッダしか見ない。ADR-0208 §3)。
// そのため calc-svc は decodeStrict の直後、format の列挙検証より前(= ID 解決・engine 呼び出し
// より前)にここで自前に検査する。値は契約の数値と揃えること。
//
// 重複の判定は解決前の生の値(*string)で行い、null(持ち物なし)も1つの値として数える
// (JSON Schema の uniqueItems が null を値として区別するのと同じ扱い。ADR-0208 §3)。

import "example.com/pokecalc/services/internal/api"

const (
	maxPresetsCount        = 8   // BulkCalcRequest.presets(DefenderPreset の enum は8値)
	maxItemVariantsCount   = 64  // BulkCalcRequest.itemVariants
	maxItemCandidatesCount = 64  // ReverseRequest.itemCandidates
	maxObservationsCount   = 16  // ReverseRequest.observations(minItems: 1 は既存どおり別で検証)
	maxCandidatesUpperOnly = 128 // ReverseRequest.maxCandidates の上限(0 は「全件」のまま)
)

// checkBulkLimits は presets / itemVariants の件数上限と itemVariants の重複を検査する。
// presets の重複は engine の sentinel(duplicate_preset)に委ねる(ADR-0208 §2。既存テストを弱めない)。
func checkBulkLimits(req api.BulkCalcRequest) error {
	if req.Presets != nil && len(*req.Presets) > maxPresetsCount {
		return newError(api.InvalidInput, "presets は %d 件以下でなければならない: %d 件", maxPresetsCount, len(*req.Presets))
	}
	return checkItemIDLimit("itemVariants", req.ItemVariants, maxItemVariantsCount)
}

// checkReverseLimits は itemCandidates / observations の件数上限、itemCandidates の重複、
// maxCandidates の範囲を検査する。
func checkReverseLimits(req api.ReverseRequest) error {
	if err := checkItemIDLimit("itemCandidates", req.ItemCandidates, maxItemCandidatesCount); err != nil {
		return err
	}
	if len(req.Observations) > maxObservationsCount {
		return newError(api.InvalidInput, "observations は %d 件以下でなければならない: %d 件", maxObservationsCount, len(req.Observations))
	}
	if req.MaxCandidates != nil {
		if v := *req.MaxCandidates; v < 0 || v > maxCandidatesUpperOnly {
			return newError(api.InvalidInput, "maxCandidates は 0..%d でなければならない: %d", maxCandidatesUpperOnly, v)
		}
	}
	return nil
}

// checkItemIDLimit は持ち物候補配列(null 許容)の件数上限と重複を検査する。
// null も1つの値として数える(この関数の doc コメント参照)。
func checkItemIDLimit(label string, ids *[]*string, max int) error {
	if ids == nil {
		return nil
	}
	list := *ids
	if len(list) > max {
		return newError(api.InvalidInput, "%s は %d 件以下でなければならない: %d 件", label, max, len(list))
	}
	seen := make(map[string]struct{}, len(list))
	seenNull := false
	for _, id := range list {
		if id == nil {
			if seenNull {
				return newError(api.InvalidInput, "%s に重複がある(null)", label)
			}
			seenNull = true
			continue
		}
		if _, dup := seen[*id]; dup {
			return newError(api.InvalidInput, "%s に重複がある: %q", label, *id)
		}
		seen[*id] = struct{}{}
	}
	return nil
}
