// Package importer は取得元のスナップショットを読み込み(LoadInput)、ADR-0100 の行に変換し
// (Convert)、pokedex の DB に冪等に投入する(Apply / Run)。ADR-0101 を参照。
//
// engine は変更しない(CLAUDE.md 絶対ルール2)。ネットワークには触らない(取得は tools/importer)。
package importer

import "errors"

// ErrInvalidInput はファイルが無い・読めない・形式違反(未知フィールド・schemaVersion・source・
// 版の食い違い・ID/日付の形式)。
var ErrInvalidInput = errors.New("importer: 入力の形式が不正")

// ErrInvalidData は入力同士が矛盾する(対応の無い calc の種族、実在しない除外名、
// formeOrder に無いフォーム、メガの持ち物が無い、除外したタイプの使用、mod の食い違い、既定が複数)。
var ErrInvalidData = errors.New("importer: 入力の内容が矛盾している")

// ErrBlocked は人間の裁定が要る食い違い(攻撃技のタイプ、技の分類・威力、種族のタイプ・種族値)。
var ErrBlocked = errors.New("importer: 人間の裁定が必要な食い違いがある")

// ErrKeyChanged は既存の showdown_id に対する species の key が投入で変わること
// (team-svc 等が保存した key を壊さないための保護)。
var ErrKeyChanged = errors.New("importer: 既存の種族の key が変わる投入")

// FindingKind は Report に載る指摘の種類(ケバブケース)。
type FindingKind string

const (
	KindMoveExcluded      FindingKind = "move-excluded"
	KindMoveShowdownOnly  FindingKind = "move-showdown-only"
	KindMoveTypeMismatch  FindingKind = "move-type-mismatch"
	KindMoveValueMismatch FindingKind = "move-value-mismatch"

	// KindItemExcluded / KindItemShowdownOnly は持ち物の取り込み判定(ADR-0101 §5「技と同じ規則」)。
	KindItemExcluded     FindingKind = "item-excluded"
	KindItemShowdownOnly FindingKind = "item-showdown-only"

	KindSpeciesMismatch     FindingKind = "species-mismatch"
	KindSpeciesShowdownOnly FindingKind = "species-showdown-only"
	KindSpeciesExcluded     FindingKind = "species-excluded"
	KindFormFolded          FindingKind = "form-folded"

	// KindAbilityShowdownOnly は、取り込んだ種族の特性スロットに現れる特性が calc の一覧に
	// 無いときの警告(ADR-0101 §5)。取り込みは止めない(calc の一覧は特性の一覧としての正
	// ではなく参考情報のため)。
	KindAbilityShowdownOnly FindingKind = "ability-showdown-only"

	KindNameFallback   FindingKind = "name-fallback"
	KindOverrideUnused FindingKind = "override-unused"
	KindEffectUnused   FindingKind = "effect-unused"
)

// Finding は1件の指摘。ID は技・持ち物・特性 ID、種族は showdown_id(calc だけのものは
// toID(calc 名))。Detail は補足情報(任意)。
type Finding struct {
	Kind   FindingKind
	ID     string
	Detail string
}

// Report は Convert の結果の指摘一覧。Blockers が1件でもあれば Output は空で返る。
type Report struct {
	Warnings []Finding
	Blockers []Finding
}
