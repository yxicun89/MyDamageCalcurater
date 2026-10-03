package httpapi

// エラーの共通の形(ADR-0200 §1.6)。calc-svc の失敗はすべて httpError に写し、
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
	// ダメージを与えられない技の逆算(issue #317)。契約に専用の code が無いので invalid_input
	// (400)に写す。専用の code の追加は API レーンへ依頼(ADR-0117 §3)。
	{engine.ErrMoveDealsNoDamage, api.InvalidInput},
	// 防御側の上書きのランク範囲外・未知の status(ADR-0216。HTTP の status は先に parseStatusCondition で弾くので届かない)。
	{engine.ErrInvalidDefenderOverride, api.InvalidInput},
	// 特性の候補が不正(件数超過・ID重複・種族が持たない特性)。engine/wasmapi と同じく invalid_input に
	// 写す(新しい code は足さない。ADR-0126 §5・ADR-0214)。マスタに無いIDはこれより前(resolveAbilityCandidates
	// でのstore参照)でunknown_abilityにする。
	{engine.ErrInvalidAbilityCandidates, api.InvalidInput},
	// 調整の入力の不正(ADR-0250 §4。wasmapi と同じく invalid_input。新しい code は足さない)。
	{engine.ErrInvalidAdjustInput, api.InvalidInput},
}

// errFromEngine は engine が返したエラーを安定した code の httpError に写す。
// sentinel に無い想定外の失敗は固定文だけをクライアントへ返し、詳細はログにだけ残す
// (critic 指摘 O4。Go の内部情報を message に出さない ADR-0200 AC-7 と同じ理由)。
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

// checkMegaItem はメガシンカ後の種族の持ち物規則(issue #315・ADR-0200 §4 追記)を検証する。
// メガ種族は requiredItemId の持ち物か持ち物なし(nil)だけを受け付け、別の持ち物は 400 invalid_input。
// メガでない種族は何も検査しない。label は入力の場所(例 "攻撃側"・"itemVariants[1]")。
func (s *Server) checkMegaItem(label string, species engine.Species, item *engine.Item) error {
	if item == nil {
		return nil
	}
	required, isMega := s.store.MegaRequiredItem(species.Key)
	if !isMega || item.ID == required {
		return nil
	}
	if required == "" {
		return newError(api.InvalidInput, "%s: メガシンカ後の種族 %q には持ち物を持たせられない(指定: %q)",
			label, species.Key, item.ID)
	}
	return newError(api.InvalidInput, "%s: メガシンカ後の種族 %q には持ち物 %q を持たせられない(持てるのはメガストーン %q だけ)",
		label, species.Key, item.ID, required)
}

// checkMegaItems は持ち物候補の各要素に checkMegaItem を当てる(bulk の itemVariants・reverse の itemCandidates)。
func (s *Server) checkMegaItems(label string, species engine.Species, items []*engine.Item) error {
	for i, it := range items {
		if err := s.checkMegaItem(fmt.Sprintf("%s[%d]", label, i), species, it); err != nil {
			return err
		}
	}
	return nil
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
