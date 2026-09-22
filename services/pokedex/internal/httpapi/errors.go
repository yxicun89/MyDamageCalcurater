package httpapi

// エラーの共通の形(ADR-0105 §2・§3)。calc-svc の httpapi(ADR-0200)と同じ語彙・同じ写し方にする。

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// messageInternal は回復した panic・想定外の失敗に付ける固定文。内部情報は出さない。
const messageInternal = "内部エラーが発生した"

// messageUnavailable はマスタ(DB)が使えないときの固定文。DB のエラー文はここに出さない(ADR-0105 §2・§3)。
const messageUnavailable = "マスタが利用できない"

// httpError は境界で検出した失敗。code は契約の ErrorCode、message は日本語の説明。
type httpError struct {
	status  int
	code    api.ErrorCode
	message string
}

func (e *httpError) Error() string { return e.message }

// newError は code から HTTP ステータスを決めて httpError を作る。
func newError(code api.ErrorCode, format string, args ...any) error {
	return &httpError{status: statusForCode(code), code: code, message: fmt.Sprintf(format, args...)}
}

// statusForCode は ErrorCode から HTTP ステータスを決める。
func statusForCode(code api.ErrorCode) int {
	switch code {
	case api.NotFound:
		return http.StatusNotFound
	case api.Internal:
		return http.StatusInternalServerError
	case api.MasterUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

// unavailable は DB の失敗・マスタの不完全さを 503 master_unavailable にする。詳細はログにだけ残す
// (ADR-0105 §2「DB のエラー文を出さない」・§3「入力の検証(400)は DB を呼ぶ前に行う」)。
func unavailable(reason string, err error) error {
	slog.Error("pokedex-svc: マスタが利用できない", "reason", reason, "error", err)
	return newError(api.MasterUnavailable, "%s", messageUnavailable)
}

// notFoundForCalc は calc-svc の担当(pokedex-svc の担当外)の操作に返す 404。
func notFoundForCalc() error {
	return newError(api.NotFound, "このサービスの担当外の操作")
}

// duplicateHeaderMessage は生成ラッパ(ServerInterfaceWrapper)が同名ヘッダを複数個
// 受け取ったときに返す echo.HTTPError.Message の断片(oapi-codegen が生成する固定の英文)。
const duplicateHeaderMessage = "Expected one value for"

// missingHeaderMessages は「ヘッダが無い/空」を意味する生成ラッパの Message の断片。
var missingHeaderMessages = []string{"is required, but not found", "is empty, can't bind"}

// httpErrorHandler は echo に渡ったエラー(Server が返した httpError・生成ラッパのヘッダ/クエリ検証・
// echo の既定 404/405 を含む)をすべて Error 形式に揃える。
func httpErrorHandler(c *echo.Context, err error) {
	if r, uerr := echo.UnwrapResponse(c.Response()); uerr == nil && r.Committed {
		return
	}
	status, body := errorBodyFor(err)
	_ = c.JSON(status, body)
}

func errorBodyFor(err error) (int, api.Error) {
	var he *httpError
	if errors.As(err, &he) {
		return he.status, api.Error{Code: he.code, Message: he.message}
	}
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		switch sc.StatusCode() {
		case http.StatusNotFound, http.StatusMethodNotAllowed:
			// ルートが無い・メソッド違い(ADR-0105 §2: メソッド違いに新しい code を足さず not_found にする)。
			return http.StatusNotFound, api.Error{Code: api.NotFound, Message: "ルートが無い"}
		case http.StatusBadRequest:
			var ee *echo.HTTPError
			if errors.As(err, &ee) {
				msg := fmt.Sprint(ee.Message)
				switch {
				case containsAny(msg, missingHeaderMessages):
					return http.StatusBadRequest, api.Error{Code: api.MissingHeader, Message: "X-Device-Id / X-Session-Id が無い"}
				case strings.Contains(msg, duplicateHeaderMessage):
					return http.StatusBadRequest, api.Error{Code: api.InvalidHeader, Message: "リクエストヘッダの指定が不正"}
				default:
					// クエリパラメータの bind 失敗(例 limit=abc)。ヘッダの失敗と区別する(ADR-0105 §3)。
					return http.StatusBadRequest, api.Error{Code: api.InvalidInput, Message: "クエリパラメータが不正"}
				}
			}
		}
	}
	// 想定外の失敗は固定文だけをクライアントへ返し、詳細はログにだけ残す。
	slog.Error("pokedex-svc: 想定外のエラー", "error", err)
	return http.StatusInternalServerError, api.Error{Code: api.Internal, Message: messageInternal}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
