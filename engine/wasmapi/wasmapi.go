// Package wasmapi は engine の計算を「JSON 文字列 → JSON 文字列」の境界として公開する。
//
// 置き場所と方針は ADR-0011:
//
//   - engine 本体パッケージ(`engine`)は変更しない。ここは engine の公開 API を呼ぶだけで、
//     独自のダメージ式・独自の丸めを持たない(CLAUDE.md 絶対ルール2・3)。
//   - build タグを持たない。ネイティブ Go でテストでき、`make test` に含まれる。
//   - syscall/js に依存しない。WASM 固有のグローバル登録は engine/cmd/wasm が行う。
//   - マスタ(ポケモン・技・持ち物・特性)を持たない。解決済みの値を JSON で受け取るだけ
//     (CLAUDE.md ドメイン規約。マスタ解決は Web 側 = P4-5 の仕事)。
//   - engine の型に json タグを足さない。境界の DTO(dto.go)を独立に定義し、変換する。
//
// 入出力の契約(フィールド名・型・列挙値・エラーコード)は ADR-0011 §3〜§5 が正で、
// 具体的なベクタは engine/wasmapi/testdata/vectors.json にある。
//
// 成功: {"result": ...}
// 失敗: {"error":{"code":"<安定した文字列>","message":"<日本語の説明>"}}
//
// いずれの関数も panic を外へ出さない。未知の失敗も必ずエラー封筒で返す。
// 返す文字列は改行で終わらない(Go/WASM のバイト一致比較のため)。
// どの関数もグローバル状態・キャッシュを持たず、同じ入力には常に同じバイト列を返す。
package wasmapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"example.com/pokecalc/engine"
)

// エラーコード。JS/WASM 境界を越えて安定させる文字列で、engine の sentinel と1対1に対応させる
// (ADR-0011 §5)。値は画面・テストが依存するので、変えるときは ADR と一緒に変える。
const (
	// CodeInvalidJSON はリクエストが JSON として壊れている / 型が合わない。
	CodeInvalidJSON = "invalid_json"
	// CodeUnknownField はリクエストに契約外のフィールドが含まれている。
	CodeUnknownField = "unknown_field"
	// CodeInvalidEnum は列挙(タイプ・分類・天候・フィールド・状態異常・ステータスキー・形式)の値が不正。
	CodeInvalidEnum = "invalid_enum"
	// CodeInvalidInput は engine の入力検証エラー(Individual.Validate 由来: SP 範囲/合計・ランク・性格・タイプ数・レベル)。
	// presets/itemVariants/itemCandidates/observations の件数上限・maxCandidates の範囲超過も
	// ここに写す(issue #110。ADR-0208 §2・ADR-0108。新しいコードは足さない)。
	CodeInvalidInput = "invalid_input"
	// CodeUnknownPreset は engine.ErrUnknownPreset。
	CodeUnknownPreset = "unknown_preset"
	// CodeDuplicatePreset は engine.ErrDuplicatePreset。
	CodeDuplicatePreset = "duplicate_preset"
	// CodeInvalidPreset は engine.ErrInvalidPreset。
	CodeInvalidPreset = "invalid_preset"
	// CodeInvalidReverseSide は engine.ErrInvalidReverseSide。
	CodeInvalidReverseSide = "invalid_reverse_side"
	// CodeNoObservation は engine.ErrNoObservation。
	CodeNoObservation = "no_observation"
	// CodeInvalidObservation は engine.ErrInvalidObservation。
	CodeInvalidObservation = "invalid_observation"
	// CodeTypeChartMissing は engine.ErrTypeChartMissing(リクエストに typeChart が無い)。
	// ADR-0011 §13 / ADR-0013。境界は既定の表を補わない。
	CodeTypeChartMissing = "type_chart_missing"
	// CodeInvalidTypeChart は engine.ErrInvalidTypeChart(表の定義が不正)。
	CodeInvalidTypeChart = "invalid_type_chart"
	// CodeUnknownType は engine.ErrUnknownType(綴りは正しいが渡された表に無いタイプ)。
	CodeUnknownType = "unknown_type"
	// CodeInternal は上記のどれにも当てはまらない失敗(回復した panic を含む)。
	CodeInternal = "internal"
)

// Calc は1対1のダメージ計算を行う。requestJSON は ADR-0011 §3 の CalcRequest。
// 戻り値は {"result": CalcResult} または {"error": ...}。engine.CalcDamage の素通しであり、
// 独自の計算をしてはならない。
func Calc(requestJSON string) string {
	return handle(requestJSON, (*calcRequest).run)
}

// CalcBulk は防御側の代表調整 × 持ち物バリアントの一括計算を行う。
// requestJSON は ADR-0011 §3 の BulkRequest。engine.CalcBulk の素通し。
func CalcBulk(requestJSON string) string {
	return handle(requestJSON, (*bulkRequest).run)
}

// CalcReverse は観測ダメージからの逆算(調整推定)を行う。
// requestJSON は ADR-0011 §3 の ReverseRequest。engine.CalcReverse の素通し。
func CalcReverse(requestJSON string) string {
	return handle(requestJSON, (*reverseRequest).run)
}

// --- 封筒 -------------------------------------------------------------------

type successEnvelope[T any] struct {
	Result T `json:"result"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// messageInternal は回復した panic に付ける固定文。Go のランタイム情報を JS へ出さない。
const messageInternal = "内部エラーが発生した"

// handle はリクエストを厳格にデコードし、run を実行して封筒に包む。panic はここで握る。
func handle[Req, Res any](requestJSON string, run func(*Req) (Res, error)) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = marshalError(CodeInternal, messageInternal)
		}
	}()

	var req Req
	if err := decodeStrict(requestJSON, &req); err != nil {
		return errorResponse(err)
	}
	res, err := run(&req)
	if err != nil {
		return errorResponse(err)
	}
	// json.Encoder は末尾に改行を付けるので使わない(Go/WASM のバイト一致比較のため)。
	b, err := json.Marshal(successEnvelope[Res]{Result: res})
	if err != nil {
		return marshalError(CodeInternal, messageInternal)
	}
	return string(b)
}

func marshalError(code, message string) string {
	// 固定の文字列2つだけを持つ構造体なので Marshal は失敗しない。
	b, _ := json.Marshal(errorEnvelope{Error: errorBody{Code: code, Message: message}})
	return string(b)
}

// boundaryError は境界で検出した失敗。code は安定した文字列、msg は日本語の説明。
type boundaryError struct {
	code string
	msg  string
}

func (e *boundaryError) Error() string { return e.msg }

func fail(code, format string, args ...any) error {
	return &boundaryError{code: code, msg: fmt.Sprintf(format, args...)}
}

// errorResponse は err を安定 code のエラー封筒に写す。engine の sentinel は errors.Is で判別する。
func errorResponse(err error) string {
	var be *boundaryError
	if errors.As(err, &be) {
		return marshalError(be.code, be.msg)
	}
	sentinels := []struct {
		err  error
		code string
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
		// 件数・範囲の上限(issue #110。ADR-0208 §2 と同じく新しいコードは足さず invalid_input に写す。
		// ADR-0108)。
		{engine.ErrTooManyPresets, CodeInvalidInput},
		{engine.ErrTooManyItemVariants, CodeInvalidInput},
		{engine.ErrTooManyItemCandidates, CodeInvalidInput},
		{engine.ErrTooManyObservations, CodeInvalidInput},
		{engine.ErrInvalidMaxCandidates, CodeInvalidInput},
		// 特性の候補の不正(issue #272。ADR-0126。新しいコードは API 契約の持ち物なので足さない)。
		{engine.ErrInvalidAbilityCandidates, CodeInvalidInput},
		// ダメージを与えられない技の逆算(issue #317。専用の code は API 契約の持ち物なので、
		// 追加されるまでは invalid_input に写す。ADR-0117 §3)。
		{engine.ErrMoveDealsNoDamage, CodeInvalidInput},
	}
	for _, s := range sentinels {
		if errors.Is(err, s.err) {
			return marshalError(s.code, err.Error())
		}
	}
	return marshalError(CodeInternal, "想定外のエラー: "+err.Error())
}

// --- デコード ---------------------------------------------------------------

// decodeStrict は未知フィールドを拒否して JSON オブジェクトを dst へ読む。
func decodeStrict(s string, dst any) error {
	if !strings.HasPrefix(strings.TrimLeft(s, " \t\r\n"), "{") {
		return fail(CodeInvalidJSON, "リクエストは JSON オブジェクトでなければならない")
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}
	// オブジェクトの後ろに余計なデータがあれば拒否する。
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fail(CodeInvalidJSON, "JSON の後ろに余計なデータがある")
	}
	return nil
}

// unknownFieldPrefix は encoding/json が DisallowUnknownFields で返すエラーの接頭辞
// (型付きエラーが無いので文字列で判別する。テストが固定している)。
const unknownFieldPrefix = "json: unknown field "

func decodeError(err error) error {
	if rest, ok := strings.CutPrefix(err.Error(), unknownFieldPrefix); ok {
		name := rest
		if u, uerr := strconv.Unquote(rest); uerr == nil {
			name = u
		}
		return fail(CodeUnknownField, "契約にないフィールド %q", name)
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return fail(CodeInvalidJSON, "フィールド %q の型が合わない(%s を期待)", typeErr.Field, typeErr.Type)
	}
	return fail(CodeInvalidJSON, "リクエストが JSON として不正: %v", err)
}
