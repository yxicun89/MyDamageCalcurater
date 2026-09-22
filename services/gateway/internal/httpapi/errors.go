package httpapi

// エラーの共通の形(ADR-0202 §7・calc-svc の internal/httpapi/errors.go と同じ流儀)。
// gateway の失敗はすべて httpError に写し、echo の HTTPErrorHandler で
// {"code","message"} の Error 本文に変換する。

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
)

// messageInternal は回復した panic・想定外の失敗に付ける固定文。
// Go のランタイム情報をクライアントへ出さない(ADR-0202 §7)。
const messageInternal = "内部エラーが発生した"

// httpError は境界で検出した失敗。code は契約の ErrorCode、message は日本語の説明。
type httpError struct {
	status  int
	code    api.ErrorCode
	message string
}

func (e *httpError) Error() string { return e.message }

// newError は code から HTTP ステータスを決めて httpError を作る(ADR-0202 のステータス対応表)。
func newError(code api.ErrorCode, format string, args ...any) error {
	return &httpError{status: statusForCode(code), code: code, message: fmt.Sprintf(format, args...)}
}

// statusForCode は gateway が返す ErrorCode から HTTP ステータスを決める(ADR-0202 §3・§4・§5・§7)。
func statusForCode(code api.ErrorCode) int {
	switch code {
	case api.NotFound:
		return http.StatusNotFound
	case api.UpstreamUnavailable:
		return http.StatusServiceUnavailable
	case api.Internal:
		return http.StatusInternalServerError
	default:
		// MissingHeader / InvalidHeader はどちらも 400。
		return http.StatusBadRequest
	}
}

// recoverMiddleware は panic を回復し、500 internal の httpError にする(スタック等を出さない。AC-G7)。
// 許可オリジンなら CORS も付ける(必須2: panic 回復の 500 で ACAO が抜け落ちる退行の修正。
// panic はまだ何も書き出していない時点[calc/pokedex/assets への転送前・転送中の RoundTrip]で
// 起きるので、ここで付けた ACAO がそのまま httpErrorHandler の応答に乗る)。
func (g *gateway) recoverMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				origin := c.Request().Header.Get("Origin")
				if g.originAllowed(origin) {
					setCORSAllowed(c.Response().Header(), origin)
				}
				err = newError(api.Internal, "%s", messageInternal)
			}
		}()
		return next(c)
	}
}

// httpErrorHandler は serve が返した httpError と echo の既定エラーをすべて Error 形式に揃える。
// serve は catch-all のワイルドカードルート("/*")だけを登録するので、echo 自身のルーティング
// エラー(404/405)は原則発生しないが、想定外の型のエラーも internal で拾う(calc-svc と同じ形)。
func httpErrorHandler(c *echo.Context, err error) {
	if r, uerr := echo.UnwrapResponse(c.Response()); uerr == nil && r.Committed {
		return
	}
	var he *httpError
	if errors.As(err, &he) {
		_ = c.JSON(he.status, api.Error{Code: he.code, Message: he.message})
		return
	}
	_ = c.JSON(http.StatusInternalServerError, api.Error{Code: api.Internal, Message: messageInternal})
}
