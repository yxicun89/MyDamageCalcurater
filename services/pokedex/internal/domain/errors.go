package domain

// 失敗の語彙(ADR-0105 §2・§3)。値は契約(api/openapi.yaml の ErrorCode)と同じ文字列にし、
// HTTP のステータスへの写しとログは adapters/httpapi が行う(ここは HTTP を知らない)。

import "fmt"

// Code は失敗の種類。値は契約の ErrorCode と同じ文字列(adapters/httpapi のテストが一致を固定する)。
type Code string

// pokedex-svc の操作が返しうる Code。
const (
	CodeInvalidInput      Code = "invalid_input"
	CodeInvalidEnum       Code = "invalid_enum"
	CodeNotFound          Code = "not_found"
	CodeMasterUnavailable Code = "master_unavailable"
	CodeInternal          Code = "internal"
)

// AllCodes は上の Code の一覧(契約の ErrorCode と一致することをテストで確かめるため)。
func AllCodes() []Code {
	return []Code{CodeInvalidInput, CodeInvalidEnum, CodeNotFound, CodeMasterUnavailable, CodeInternal}
}

// MessageUnavailable はマスタ(DB)が使えないときの固定文。DB のエラー文はここに出さない(ADR-0105 §2・§3)。
const MessageUnavailable = "マスタが利用できない"

// Error は入力の検証・マスタの読み出しの失敗。Message は日本語の説明(そのまま応答に載る)。
// Reason と Cause は master_unavailable の詳細で、応答には出さずログにだけ残す(adapters が出す)。
type Error struct {
	Code    Code
	Message string
	Reason  string
	Cause   error
}

func (e *Error) Error() string { return e.Message }

// Errorf は Code と説明から Error を作る。
func Errorf(code Code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Unavailable は DB の失敗・マスタの不完全さを master_unavailable にする。reason と cause はログ用
// (ADR-0105 §2「DB のエラー文を出さない」)。
func Unavailable(reason string, cause error) error {
	return &Error{Code: CodeMasterUnavailable, Message: MessageUnavailable, Reason: reason, Cause: cause}
}

// ErrEmpty は「未投入」を表す(内部の判定用。応答には出さず master_unavailable に写す)。
type ErrEmpty string

func (e ErrEmpty) Error() string { return "empty: " + string(e) }
