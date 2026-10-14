package httpapi

// エラーの共通の形(ADR-0200 §1.6)。calc-svc の失敗はすべて httpError に写し、
// echo の HTTPErrorHandler で {"code","message"} の Error 本文に変換する。
// engine の sentinel → code の写像は wasmapi と同じ語彙(ADR-0011 §5)で、domain.FromEngine が持つ。

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

	"example.com/pokecalc/services/calc/internal/app"
	"example.com/pokecalc/services/calc/internal/domain"
	"example.com/pokecalc/services/internal/api"
)

// httpError は境界で検出した失敗。code は契約の ErrorCode、message は日本語の説明。
type httpError struct {
	status  int
	code    api.ErrorCode
	message string
	// retryAfter は応答に Retry-After: 1 を付ける(締め切り超過の 503。guard の過負荷 503 と揃える)。
	retryAfter bool
}

func (e *httpError) Error() string { return e.message }

// newError は code から HTTP ステータスを決めて httpError を作る(ADR-0200 のステータス対応表)。
func newError(code api.ErrorCode, format string, args ...any) error {
	return &httpError{status: statusForCode(code), code: code, message: fmt.Sprintf(format, args...)}
}

// statusForCode は ErrorCode から HTTP ステータスを決める(ADR-0200 §1.6)。
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

// errFromEngine は engine が返したエラーを安定した code の httpError に写す(写像は domain.FromEngine)。
// sentinel に無い想定外の失敗は固定文だけをクライアントへ返し、詳細はログにだけ残す
// (critic 指摘 O4。Go の内部情報を message に出さない ADR-0200 AC-7 と同じ理由)。
func errFromEngine(err error) error {
	return httpErrorFrom(domain.FromEngine(err))
}

// httpErrorFrom は app・domain の失敗を httpError に写す(nil は nil)。
// domain.Code は契約の ErrorCode と同じ文字列(app_mapping_test の TestDomainCodesAreContractErrorCodes が固定する)。
// app・domain のどちらの語彙でもない失敗は、errorBodyFor の既定と同じく固定文の internal(500)にし、詳細はログにだけ残す。
func httpErrorFrom(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, app.ErrDeadlineExceeded) {
		return errDeadlineExceeded()
	}
	var de *domain.Error
	if errors.As(err, &de) {
		if de.Cause != nil {
			// Cause を持つのは domain.FromEngine の想定外の失敗だけ。
			slog.Error("calc-svc: engine から想定外のエラー", "error", de.Cause)
		}
		return newError(api.ErrorCode(de.Code), "%s", de.Message)
	}
	slog.Error("calc-svc: 想定外のエラー", "error", err)
	return newError(api.Internal, "%s", messageInternal)
}

// --- 厳格デコード(engine/wasmapi の decodeStrict と同じ振る舞い) --------------

// decodeStrict は未知フィールドを拒否して JSON オブジェクトを dst へ読む。
// individualKeys は本文の直下にある Individual のキー名(attacker / defender / known)で、
// それぞれの sp(StatBlock の6キー)が契約どおりそろっていることも確かめる(issue #316。
// 生成型は値型でゼロ値が入るため、キーの有無は生の JSON で見る。ADR-0200 §4)。
func decodeStrict(r io.Reader, dst any, individualKeys ...string) error {
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
	return requireIndividualSP(data, individualKeys)
}

// statBlockKeys は契約の StatBlock の必須6キー。
var statBlockKeys = []string{"hp", "atk", "def", "spa", "spd", "spe"}

// requireIndividualSP は、本文直下の各 Individual の sp が存在し、6キーすべてを(null でなく)持つことを
// 確かめる(欠落は invalid_input。judge-svc の toStats と同じ扱い)。Individual 自体の欠落は
// ここでは見ない(speciesKey 空として既存どおり unknown_species になる)。encoding/json は
// キー名の大文字小文字を区別せず束縛するので("Attacker" は attacker に入る)、ここでも EqualFold で探す。
func requireIndividualSP(data []byte, individualKeys []string) error {
	if len(individualKeys) == 0 {
		return nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil // decodeStrict が構文は検証済み
	}
	for _, key := range individualKeys {
		rawInd, ok := lookupFold(top, key)
		if !ok {
			continue
		}
		var ind map[string]json.RawMessage
		if json.Unmarshal(rawInd, &ind) != nil || ind == nil {
			continue
		}
		var sp map[string]json.RawMessage
		if raw, ok := lookupFold(ind, "sp"); !ok || json.Unmarshal(raw, &sp) != nil || sp == nil {
			return newError(api.InvalidInput, "%s.sp が必須(能力ポイントの6キーをすべて指定する)", key)
		}
		for _, k := range statBlockKeys {
			if raw, ok := lookupFold(sp, k); !ok || string(bytes.TrimSpace(raw)) == "null" {
				return newError(api.InvalidInput, "%s.sp.%s が必須(能力ポイントの6キーをすべて指定する)", key, k)
			}
		}
	}
	return nil
}

// lookupFold は大文字小文字を区別せずキーを探す(encoding/json の束縛と同じ)。
func lookupFold(m map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
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
