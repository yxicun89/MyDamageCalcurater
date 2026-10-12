package domain

// 失敗の語彙(ADR-0254。calc-svc の domain と同じ流儀。ADR-0252)。値は契約(api/openapi.yaml の ErrorCode)と
// 同じ文字列にし、HTTP のステータスへの写しは adapters/httpapi が行う(ここは HTTP を知らない)。

import "fmt"

// Code は失敗の種類。値は契約の ErrorCode と同じ文字列(adapters/httpapi のテストが一致を固定する)。
type Code string

// record の検証が返しうる Code。
const (
	CodeInvalidInput Code = "invalid_input"
	CodeInvalidEnum  Code = "invalid_enum"
)

// AllCodes は上の Code の一覧(契約の ErrorCode と一致することをテストで確かめるため)。
func AllCodes() []Code {
	return []Code{CodeInvalidInput, CodeInvalidEnum}
}

// Error は入力の検証の失敗。Message は日本語の説明(そのまま応答に載る)。
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return e.Message }

// Errorf は Code と説明から Error を作る。
func Errorf(code Code, format string, args ...any) error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
