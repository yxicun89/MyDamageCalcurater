package httpapi

// エラーの共通の形(ADR-0016 §1.6)。calc-svc の失敗はすべて httpError に写し、
// echo の HTTPErrorHandler で {"code","message"} の Error 本文に変換する。
// engine の sentinel → code の写像は wasmapi と同じ語彙(ADR-0011 §5)を使う。

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

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/api"
)

// httpError は境界で検出した失敗。code は契約の ErrorCode、message は日本語の説明。
type httpError struct {
	status  int
	code    api.ErrorCode
	message string
}

func (e *httpError) Error() string { return e.message }

// newError は code から HTTP ステータスを決めて httpError を作る(ADR-0016 のステータス対応表)。
func newError(code api.ErrorCode, format string, args ...any) error {
	return &httpError{status: statusForCode(code), code: code, message: fmt.Sprintf(format, args...)}
}

// statusForCode は ErrorCode から HTTP ステータスを決める(ADR-0016 §1.6)。
// 入力の不正と ID 不明はすべて 400、not_found は 404、internal は 500、
// master_unavailable / upstream_unavailable は 503。
// type_chart_missing / invalid_type_chart は HTTP では常に起動時に読み込んだ Store(マスタ)
// 側の不備であり、クライアントの入力起因では起こらないため 500 にする(critic 指摘 O3。
// WASM 境界はリクエストに typeChart を乗せるので 400 のまま。ADR-0011)。
func statusForCode(code api.ErrorCode) int {
	switch code {
	case api.NotFound:
		return http.StatusNotFound
	case api.Internal, api.TypeChartMissing, api.InvalidTypeChart:
		return http.StatusInternalServerError
	case api.MasterUnavailable, api.UpstreamUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

// engineSentinels は engine の sentinel エラーと ErrorCode の対応(wasmapi.errorResponse と同じ語彙)。
var engineSentinels = []struct {
	err  error
	code api.ErrorCode
}{
	{engine.ErrUnknownPreset, api.UnknownPreset},
	{engine.ErrDuplicatePreset, api.DuplicatePreset},
	{engine.ErrInvalidPreset, api.InvalidPreset},
	{engine.ErrInvalidReverseSide, api.InvalidReverseSide},
	{engine.ErrNoObservation, api.NoObservation},
	{engine.ErrInvalidObservation, api.InvalidObservation},
	{engine.ErrTypeChartMissing, api.TypeChartMissing},
	{engine.ErrInvalidTypeChart, api.InvalidTypeChart},
	{engine.ErrUnknownType, api.UnknownType},
}

// errFromEngine は engine が返したエラーを安定した code の httpError に写す。
// sentinel に無い想定外の失敗は固定文だけをクライアントへ返し、詳細はログにだけ残す
// (critic 指摘 O4。Go の内部情報を message に出さない ADR-0016 AC-7 と同じ理由)。
func errFromEngine(err error) error {
	for _, s := range engineSentinels {
		if errors.Is(err, s.err) {
			return newError(s.code, "%v", err)
		}
	}
	slog.Error("calc-svc: engine から想定外のエラー", "error", err)
	return newError(api.Internal, "%s", messageInternal)
}

// validateIndividual は engine.Individual.Validate の失敗を invalid_input にする(wasmapi と同じ)。
func validateIndividual(label string, in engine.Individual) error {
	if err := in.Validate(); err != nil {
		return newError(api.InvalidInput, "%s の入力が不正: %v", label, err)
	}
	return nil
}

// --- 厳格デコード(engine/wasmapi の decodeStrict と同じ振る舞い) --------------

// decodeStrict は未知フィールドを拒否して JSON オブジェクトを dst へ読む。
func decodeStrict(r io.Reader, dst any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return newError(api.InvalidJson, "リクエスト本文を読めない: %v", err)
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
	// オブジェクトの後ろに余計なデータがあれば拒否する。
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return newError(api.InvalidJson, "JSON の後ろに余計なデータがある")
	}
	return nil
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
