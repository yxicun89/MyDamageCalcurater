package domain

// 失敗の語彙(ADR-0200 §1.6・ADR-0217)。値は契約(api/openapi.yaml の ErrorCode)と同じ文字列にし、
// HTTP のステータスへの写しは adapters/httpapi が行う(ここは HTTP を知らない)。

import (
	"errors"
	"fmt"

	"example.com/pokecalc/engine"
)

// Code は失敗の種類。値は契約の ErrorCode と同じ文字列(adapters/httpapi のテストが一致を固定する)。
type Code string

// calc の計算・一括・逆算が返しうる Code。
const (
	CodeInvalidInput       Code = "invalid_input"
	CodeInvalidEnum        Code = "invalid_enum"
	CodeUnknownSpecies     Code = "unknown_species"
	CodeUnknownNature      Code = "unknown_nature"
	CodeUnknownAbility     Code = "unknown_ability"
	CodeUnknownItem        Code = "unknown_item"
	CodeUnknownMove        Code = "unknown_move"
	CodeNoObservation      Code = "no_observation"
	CodeInvalidObservation Code = "invalid_observation"
	CodeUnknownPreset      Code = "unknown_preset"
	CodeDuplicatePreset    Code = "duplicate_preset"
	CodeInvalidPreset      Code = "invalid_preset"
	CodeInvalidReverseSide Code = "invalid_reverse_side"
	CodeTypeChartMissing   Code = "type_chart_missing"
	CodeInvalidTypeChart   Code = "invalid_type_chart"
	CodeUnknownType        Code = "unknown_type"
	CodeInternal           Code = "internal"
)

// AllCodes は上の Code の一覧(契約の ErrorCode と一致することをテストで確かめるため)。
func AllCodes() []Code {
	return []Code{
		CodeInvalidInput, CodeInvalidEnum, CodeUnknownSpecies, CodeUnknownNature, CodeUnknownAbility,
		CodeUnknownItem, CodeUnknownMove, CodeNoObservation, CodeInvalidObservation, CodeUnknownPreset,
		CodeDuplicatePreset, CodeInvalidPreset, CodeInvalidReverseSide, CodeTypeChartMissing,
		CodeInvalidTypeChart, CodeUnknownType, CodeInternal,
	}
}

// MessageInternal は想定外の失敗に付ける固定文。Go のランタイム情報をクライアントへ出さない(ADR-0200 AC-7)。
const MessageInternal = "内部エラーが発生した"

// Error は入力の検証・解決・engine 呼び出しの失敗。Message は日本語の説明(そのまま応答に載る)。
// Cause は想定外の失敗の元で、応答には出さずログにだけ残す(critic 指摘 O4)。
type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string { return e.Message }

// Errorf は Code と説明から Error を作る。
func Errorf(code Code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// engineSentinels は engine の sentinel エラーと Code の対応(engine/wasmapi.errorResponse と同じ語彙。ADR-0011 §5)。
var engineSentinels = []struct {
	err  error
	code Code
}{
	{engine.ErrUnknownPreset, CodeUnknownPreset},
	{engine.ErrDuplicatePreset, CodeDuplicatePreset},
	{engine.ErrInvalidPreset, CodeInvalidPreset},
	{engine.ErrInvalidReverseSide, CodeInvalidReverseSide},
	{engine.ErrNoObservation, CodeNoObservation},
	{engine.ErrInvalidObservation, CodeInvalidObservation},
	{engine.ErrTypeChartMissing, CodeTypeChartMissing},
	{engine.ErrInvalidTypeChart, CodeInvalidTypeChart},
	{engine.ErrUnknownType, CodeUnknownType},
	// ダメージを与えられない技の逆算(issue #317)。契約に専用の code が無いので invalid_input
	// (400)に写す。専用の code の追加は API レーンへ依頼(ADR-0117 §3)。
	{engine.ErrMoveDealsNoDamage, CodeInvalidInput},
	// 防御側の上書きのランク範囲外・未知の status(ADR-0216。HTTP の status は先に ParseStatus で弾くので届かない)。
	{engine.ErrInvalidDefenderOverride, CodeInvalidInput},
	// 特性の候補が不正(件数超過・ID重複・種族が持たない特性)。engine/wasmapi と同じく invalid_input に
	// 写す(新しい code は足さない。ADR-0126 §5・ADR-0214)。マスタに無いIDはこれより前(特性の候補の解決)で
	// unknown_ability にする。
	{engine.ErrInvalidAbilityCandidates, CodeInvalidInput},
	// 調整の入力の不正(ADR-0250 §4。wasmapi と同じく invalid_input。新しい code は足さない)。
	{engine.ErrInvalidAdjustInput, CodeInvalidInput},
	// 対戦の状態の値域外(最大 HP を超える残り HP・範囲の多段でない技の回数・範囲外の回数。ADR-0144 §6)。
	{engine.ErrInvalidBattleState, CodeInvalidInput},
	// 連続技の入力の不正(ランク変化の無い変化技の手・変化技の相手の技等。ADR-0253)と、素早さの入力の範囲外(ADR-0179)。
	{engine.ErrInvalidComboInput, CodeInvalidInput},
	{engine.ErrInvalidSpeedInput, CodeInvalidInput},
}

// FromEngine は engine が返したエラーを安定した Code の Error に写す。
// sentinel に無い想定外の失敗は固定文(MessageInternal)にし、元のエラーは Cause に残す
// (ログに出すのは adapters の仕事。Go の内部情報を応答に出さない。ADR-0200 AC-7)。
func FromEngine(err error) error {
	for _, s := range engineSentinels {
		if errors.Is(err, s.err) {
			return Errorf(s.code, "%v", err)
		}
	}
	return &Error{Code: CodeInternal, Message: MessageInternal, Cause: err}
}
