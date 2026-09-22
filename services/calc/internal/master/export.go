package master

// MasterExport(pokedex-svc の内部 API GET /internal/pokedex/master の本文。ADR-0204)から Store を作る入口と、
// その入手元(Source)。spec-writer のスタブ: 振る舞いは export_test.go / source_test.go / contract_test.go が固定する。
// implementer が実装する(暫定スナップショット形式 LoadSnapshot / LoadTypeChart / New / Snapshot は ADR-0204 で廃止)。

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"example.com/pokecalc/services/internal/api"
)

// ロード・取得の失敗。呼び出し側は errors.Is で判別する。
var (
	// ErrInvalidMaster はマスタ一式が不正(壊れた JSON・未知のフィールド・後続のデータ・schemaVersion が 1 でない・
	// dataVersion が空・ID の重複・参照先が無い・性格の不正、および共通マスタの写像のエラー)。
	// 共通マスタ(services/internal/master)の ErrInvalidRow / ErrInvalidEffect、engine.ErrInvalidTypeChart も
	// 同時に errors.Is で判別できるように包む。
	ErrInvalidMaster = errors.New("マスタ一式が不正")
	// ErrMasterUnavailable はマスタ一式を取得できない(接続できない・タイムアウト・200 以外の応答)。
	ErrMasterUnavailable = errors.New("マスタ一式を取得できない")
)

// errNotImplemented は spec-writer のスタブが返すエラー(implementer が実装したら消す)。
var errNotImplemented = errors.New("未実装(ADR-0204)")

// FromExport はマスタ一式を検証し、engine の型にしてメモリに持つ Store を作る(ADR-0204 §2)。
// 相性表・種族・技・持ち物・特性は共通マスタ(services/internal/master)の写像で engine の型にする。
// 性格は calc-svc 側で検証する(ID が空・重複、plus/minus が StatKey でない・HP を指す)。
// 不正はすべて ErrInvalidMaster で包む(部分的な Store は返さない)。
func FromExport(export api.MasterExport) (*MemoryStore, error) {
	return nil, errNotImplemented
}

// DecodeExport はマスタ一式の JSON を厳格に読む(未知のフィールド・後続のデータを拒否する。
// 効果定義の数値は字面のまま保つ: 5324.0 を 5324 に丸めて通さない)。不正は ErrInvalidMaster で包む。
func DecodeExport(r io.Reader) (api.MasterExport, error) {
	return api.MasterExport{}, errNotImplemented
}

// DataVersion はマスタ一式の dataVersion(記録とログに使うだけ。計算には影響しない)。
func (s *MemoryStore) DataVersion() string {
	return ""
}

// Source はマスタ一式の入手元(ADR-0204 §2)。
type Source interface {
	// Fetch はマスタ一式を1度取得する。ctx が終わったら ctx のエラーを包んで返す。
	Fetch(ctx context.Context) (api.MasterExport, error)
}

// FileSource は JSON ファイル(MasterExport の形)から読む入手元(k3d の local overlay と make dev 用)。
type FileSource struct {
	// Path は MasterExport の JSON ファイルのパス。
	Path string
}

var _ Source = FileSource{}

// Fetch は Source を実装する。ファイルが無い・読めない場合はそのエラーを、形が不正なら ErrInvalidMaster を包んで返す。
func (f FileSource) Fetch(ctx context.Context) (api.MasterExport, error) {
	return api.MasterExport{}, errNotImplemented
}

// HTTPSource は pokedex-svc の内部 API(GET {BaseURL}/internal/pokedex/master)から取得する入手元。
type HTTPSource struct {
	baseURL string
	client  *http.Client
}

var _ Source = (*HTTPSource)(nil)

// NewHTTPSource は pokedex-svc のベース URL(http / https の絶対 URL。末尾の / は有っても無くてもよい)と、
// 1回の取得のタイムアウト(正の値)から HTTPSource を作る。URL・タイムアウトが不正ならエラー。
func NewHTTPSource(baseURL string, timeout time.Duration) (*HTTPSource, error) {
	return nil, errNotImplemented
}

// Fetch は Source を実装する。接続できない・タイムアウト・200 以外は ErrMasterUnavailable、
// 本文が不正(壊れた JSON・未知のフィールド)なら ErrInvalidMaster を包んで返す。
func (h *HTTPSource) Fetch(ctx context.Context) (api.MasterExport, error) {
	return api.MasterExport{}, errNotImplemented
}
