package wasmapi

// メガシンカ後の種族の持ち物規則(issue #505。ADR-0321。calc-svc の checkMegaItem と同じ規則・同じ文言)。
//
// メガ種族(speciesDTO.IsMega)は RequiredItemID の持ち物か持ち物なし(nil)だけを受け付け、別の持ち物は
// invalid_input。メガでない種族は何も検査しない。メガ種族の一覧もメガストーンの一覧も持たず、
// リクエストの種族が持つ isMega・requiredItemId だけを見る(CLAUDE.md: マスタをハードコードしない)。
// engine.Species には持ち込まない(engine は変更しない)。

import "fmt"

// checkMegaItem は種族 s に持ち物 item を持たせてよいかを検証する。label は入力の場所(例 "攻撃側"・"itemVariants[1]")。
func (s speciesDTO) checkMegaItem(label string, item *itemDTO) error {
	if item == nil || !s.IsMega || item.ID == s.RequiredItemID {
		return nil
	}
	if s.RequiredItemID == "" {
		return fail(CodeInvalidInput, "%s: メガシンカ後の種族 %q には持ち物を持たせられない(指定: %q)",
			label, s.Key, item.ID)
	}
	return fail(CodeInvalidInput, "%s: メガシンカ後の種族 %q には持ち物 %q を持たせられない(持てるのはメガストーン %q だけ)",
		label, s.Key, item.ID, s.RequiredItemID)
}

// checkMegaItems は持ち物候補の各要素に checkMegaItem を当てる(bulk の itemVariants・reverse の itemCandidates)。
func (s speciesDTO) checkMegaItems(label string, items []*itemDTO) error {
	for i, it := range items {
		if err := s.checkMegaItem(fmt.Sprintf("%s[%d]", label, i), it); err != nil {
			return err
		}
	}
	return nil
}
