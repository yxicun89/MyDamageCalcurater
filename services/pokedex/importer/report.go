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

	// KindSpeciesMegaBaseDependency は、使用可能なメガの基本種が Showdown で isNonstandard
	// (calc に対応が無い)ため、base_species_key の外部キーを満たすためだけに依存行として
	// 取り込んだときの案内(止めない。ADR-0101 §5・ADR-0103 §12)。取り込みはするがレギュレーションの
	// 使用可能集合には入れないので、件数で見えるようにする。
	KindSpeciesMegaBaseDependency FindingKind = "species-mega-base-dependency"

	// KindAbilityShowdownOnly は、取り込んだ種族の特性スロットに現れる特性が calc の一覧に
	// 無いときの警告(ADR-0101 §5)。取り込みは止めない(calc の一覧は特性の一覧としての正
	// ではなく参考情報のため)。
	KindAbilityShowdownOnly FindingKind = "ability-showdown-only"

	KindNameFallback   FindingKind = "name-fallback"
	KindOverrideUnused FindingKind = "override-unused"
	KindEffectUnused   FindingKind = "effect-unused"

	// KindNatureMismatch は性格(natures)の補正が Showdown と calc で食い違う、または片方にしか
	// 無いとき(ADR-0105 §4)。ID は Showdown の ID か、calc にしかない性格名の toID。
	KindNatureMismatch FindingKind = "nature-mismatch"

	// 以下は Reconcile が追加する指摘の種類(ADR-0103 §9)。

	// KindVerdictMismatch は P2-1c の裁定(件数・ID集合のハッシュ)と実データが食い違ったとき。
	// ID は裁定の区分キー(calcOnlyExcluded / showdownOnlyIncluded / statusTypeMismatch)。
	KindVerdictMismatch FindingKind = "verdict-mismatch"
	// KindVerdictBasisChanged は裁定を行った版(basis)と実際に取り込む版が違うとき(警告だけ)。
	// ID は source 名。
	KindVerdictBasisChanged FindingKind = "verdict-basis-changed"
	// KindEffectMissing はダメージに効くハンドラを持つのに効果定義が無いとき。ID は持ち物/特性 ID。
	KindEffectMissing FindingKind = "effect-missing"
	// KindEffectNoHook は効果定義があるのにダメージに効くハンドラが無いとき。ID は持ち物/特性 ID。
	KindEffectNoHook FindingKind = "effect-no-hook"
	// KindFormLearnetDiff は畳んだフォームの習得技(取り込む技に絞る)が代表と違うとき。ID は畳んだフォーム。
	KindFormLearnsetDiff FindingKind = "form-learnset-diff"
)

// Finding は1件の指摘。ID は技・持ち物・特性 ID、種族は showdown_id(calc だけのものは
// toID(calc 名))。Detail は補足情報(任意)。
type Finding struct {
	Kind   FindingKind `json:"kind"`
	ID     string      `json:"id"`
	Detail string      `json:"detail,omitempty"`
}

// Report は Convert の結果の指摘一覧。Blockers が1件でもあれば Output は空で返る。
type Report struct {
	Warnings []Finding `json:"warnings"`
	Blockers []Finding `json:"blockers"`
}
