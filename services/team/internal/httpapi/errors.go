package httpapi

// エラーの共通の形(record-svc の services/record/internal/httpapi/errors.go と同じ流儀)。
// team-svc の失敗はすべて httpError に写し、echo の HTTPErrorHandler で {"code","message"} の
// Error 本文に変換する。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/team/internal/store"
)

// messageInternal は回復した panic・想定外の失敗に付ける固定文(Go の内部情報を出さない)。
const messageInternal = "内部エラーが発生した"

// maxRequestBodyBytes はリクエスト本文の上限(calc-svc/record-svc と同じ。無制限に読み込まない)。
const maxRequestBodyBytes = 1 << 20 // 1MiB

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

// statusForCode は ErrorCode から HTTP ステータスを決める(ADR-0200・ADR-0209 §5.3)。
func statusForCode(code api.ErrorCode) int {
	switch code {
	case api.NotFound:
		return http.StatusNotFound
	case api.Internal:
		return http.StatusInternalServerError
	case api.StoreUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

// errFromStore は store のエラーを httpError に写す(store.ErrUnavailable は 503 store_unavailable、
// それ以外は 500 internal)。DB のエラー文はログにだけ残し、クライアントへの応答には出さない
// (AC-L1)。
func errFromStore(deviceID string, err error) error {
	if errors.Is(err, store.ErrUnavailable) {
		slog.Warn("team-svc: store に届かない", "deviceId", deviceID, "code", api.StoreUnavailable, "error", err)
		return newError(api.StoreUnavailable, "保存データの DB を利用できない")
	}
	slog.Error("team-svc: store から想定外のエラー", "deviceId", deviceID, "error", err)
	return newError(api.Internal, "%s", messageInternal)
}

// notFoundForOtherServices は team-svc の担当外(calc・pokedex・record・internal)の操作に返す 404。
func notFoundForOtherServices() error {
	return newError(api.NotFound, "このサービスの担当外の操作")
}

// checkHeaders は X-Device-Id / X-Session-Id の欠落・空を missing_header にする
// (生成ラッパが先に検証するので通常はここに来ないが、念のため二重に確かめる)。
func checkHeaders(deviceID, sessionID string) error {
	if deviceID == "" || sessionID == "" {
		return newError(api.MissingHeader, "X-Device-Id / X-Session-Id が無い")
	}
	return nil
}

// deviceIDQueryNames は端末 ID をクエリで受け取ることを禁じる名前(ADR-0209 §2・§6-4)。
var deviceIDQueryNames = []string{"deviceId", "device_id"}

// checkNoDeviceIDInQuery はクエリに deviceId/device_id が含まれていたら 400 unknown_field にする。
func checkNoDeviceIDInQuery(query map[string][]string) error {
	for _, name := range deviceIDQueryNames {
		if _, ok := query[name]; ok {
			return newError(api.UnknownField, "端末 ID はヘッダでだけ受け取る(クエリの %q は使えない)", name)
		}
	}
	return nil
}

// limitedBody はリクエスト本文を maxRequestBodyBytes に制限した Reader にする。
func limitedBody(body io.Reader) io.Reader {
	return io.LimitReader(body, maxRequestBodyBytes+1)
}

// decodeStrict は未知フィールドを拒否して JSON オブジェクトを dst へ読む(calc-svc の
// decodeStrict と同じ振る舞い。ネストした構造体にも DisallowUnknownFields が効く)。
func decodeStrict(r io.Reader, dst any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return newError(api.InvalidJson, "リクエスト本文を読めない: %v", err)
	}
	if len(data) > maxRequestBodyBytes {
		return newError(api.InvalidJson, "リクエスト本文が大きすぎる")
	}
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return newError(api.InvalidJson, "リクエストは JSON オブジェクトでなければならない")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeJSONError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return newError(api.InvalidJson, "JSON の後ろに余計なデータがある")
	}
	return nil
}

// decodeNoBody は「本文を持たない」操作(GET/DELETE の各操作)のボディを検証する。
// 空・空白だけなら何もしない。JSON オブジェクトが送られてきたら、フィールドは1つも許されていない
// (契約に requestBody が無い)ので、どのフィールドでも 400 unknown_field にする(ADR-0209 §6-4)。
func decodeNoBody(body io.Reader) error {
	data, err := io.ReadAll(limitedBody(body))
	if err != nil {
		return newError(api.InvalidJson, "リクエスト本文を読めない: %v", err)
	}
	if len(data) > maxRequestBodyBytes {
		return newError(api.InvalidJson, "リクエスト本文が大きすぎる")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var empty struct{}
	if err := dec.Decode(&empty); err != nil {
		return decodeJSONError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return newError(api.InvalidJson, "JSON の後ろに余計なデータがある")
	}
	return nil
}

// headerParamNames は生成ラッパ(ServerInterfaceWrapper)が返す echo.HTTPError のメッセージに
// 含まれるヘッダ名(ヘッダ絡みの失敗かクエリ絡みの失敗かを見分けるために使う)。
var headerParamNames = []string{"X-Device-Id", "X-Session-Id"}

// duplicateHeaderMessage は同名ヘッダを複数個受け取ったときに oapi-codegen が返す固定の英文の断片
// (calc-svc/record-svc の duplicateHeaderMessage と同じ)。
const duplicateHeaderMessage = "Expected one value for"

// missingHeaderMessage は必須ヘッダが無いときに oapi-codegen が返す固定の英文の断片。
const missingHeaderMessage = "is required, but not found"

// errorBodyFor は echo に渡ったエラー(Server が返した httpError・生成ラッパのヘッダ/クエリ検証・
// echo の既定 404/405 を含む)を Error 形式(ステータス・code・message)に写す。
func errorBodyFor(err error) (int, api.Error) {
	var he *httpError
	if errors.As(err, &he) {
		return he.status, api.Error{Code: he.code, Message: he.message}
	}
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		switch sc.StatusCode() {
		case http.StatusNotFound, http.StatusMethodNotAllowed:
			// ルートが無い・メソッドが違う(ADR-0200: メソッド違いに新しい code を足さず not_found にする)。
			return http.StatusNotFound, api.Error{Code: api.NotFound, Message: "ルートが無い"}
		case http.StatusBadRequest:
			var ee *echo.HTTPError
			if errors.As(err, &ee) {
				switch {
				case strings.Contains(ee.Message, missingHeaderMessage):
					return http.StatusBadRequest, api.Error{Code: api.MissingHeader, Message: "X-Device-Id / X-Session-Id が無い"}
				case strings.Contains(ee.Message, duplicateHeaderMessage) && containsAny(ee.Message, headerParamNames):
					slog.Warn("team-svc: ヘッダが重複している", "message", ee.Message)
					return http.StatusBadRequest, api.Error{Code: api.InvalidHeader, Message: "リクエストヘッダの指定が不正"}
				default:
					return http.StatusBadRequest, api.Error{Code: api.InvalidInput, Message: "リクエストパラメータが不正"}
				}
			}
		}
	}
	// 想定外の失敗は固定文だけをクライアントへ返し、詳細はログにだけ残す。
	slog.Error("team-svc: 想定外のエラー", "error", err)
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

// unknownFieldPrefix は encoding/json が DisallowUnknownFields で返すエラーの接頭辞。
const unknownFieldPrefix = "json: unknown field "

func decodeJSONError(err error) error {
	if rest, ok := strings.CutPrefix(err.Error(), unknownFieldPrefix); ok {
		name := rest
		if u, uerr := strconv.Unquote(rest); uerr == nil {
			name = u
		}
		return newError(api.UnknownField, "契約にないフィールド %q", name)
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return newError(api.InvalidJson, "フィールド %q の型が合わない(%s を期待)", typeErr.Field, typeErr.Type)
	}
	return newError(api.InvalidJson, "リクエストが JSON として不正: %v", err)
}
