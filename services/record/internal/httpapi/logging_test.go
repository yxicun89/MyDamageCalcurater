package httpapi

// ログ規約(ADR-0209 §3・coding-rules §1)の受け入れテスト。AC-L1。
//
// 出してよい: 端末 ID・セッション ID・操作名・件数・所要時間・エラーコード。
// 出してはいけない: 個体の中身(種族・技・持ち物・特性・性格・SP)、ダメージの数値、
// 構築名・ニックネーム、お気に入りの中身、リクエスト / レスポンスの本文そのもの。
//
// record-svc は `slog.Default()` に書く(calc-svc・pokedex-svc と同じ)。テストは既定のロガーを
// バッファに差し替えて、出た行の中身を検査する。

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/record/internal/store"
)

// captureLogs は slog の既定ロガーをバッファに差し替え、テストの終わりに戻す。
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// AC-L1: 成功・失敗のどちらの経路でも、禁止された値がログに出ない。
func TestLogsDoNotLeakPayload(t *testing.T) {
	const (
		secretSpecies  = "9002-000"
		secretNickname = "テストニックネーム"
		secretDamage   = "123456"
	)

	st := newFakeStore()
	st.mu.Lock()
	d := st.state(deviceA)
	d.aggregates = []store.FrequentOpponent{{SpeciesKey: secretSpecies, Score: 2, Count: 2, LastCalculatedAt: st.now}}
	d.events = []store.CalcEvent{{
		EventID: "e1", DeviceID: deviceA, DefenderSpeciesKey: secretSpecies,
		Payload: []byte(`{"nickname":"` + secretNickname + `","maxDamage":` + secretDamage + `}`),
	}}
	st.mu.Unlock()

	buf := captureLogs(t)
	h := NewHandler(st)

	serve(t, h, http.MethodGet, pathFrequent, headers(deviceA), nil)
	serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil)

	// 失敗の経路(DB 到達不能)も見る。
	st.unavailable = true
	serve(t, h, http.MethodGet, pathFrequent, headers(deviceA), nil)

	logs := buf.String()
	for _, forbidden := range []string{secretSpecies, secretNickname, secretDamage, `"nickname"`} {
		if strings.Contains(logs, forbidden) {
			t.Errorf("ログに出してはいけない値 %q が含まれている(ADR-0209 §3):\n%s", forbidden, logs)
		}
	}
}

// 出してよい項目は出ていること(ログを空にして AC-L1 を通す抜け道を塞ぐ)。
// 失敗の経路では、少なくとも端末 ID とエラーコードが分かること。
func TestLogsKeepAllowedFields(t *testing.T) {
	st := newFakeStore()
	st.unavailable = true
	buf := captureLogs(t)

	serve(t, NewHandler(st), http.MethodGet, pathFrequent, headers(deviceA), nil)

	logs := buf.String()
	if logs == "" {
		t.Fatal("DB 到達不能なのにログが1行も出ていない(原因を追えない)")
	}
	for _, want := range []string{deviceA, "store_unavailable"} {
		if !strings.Contains(logs, want) {
			t.Errorf("ログに %q が無い(ADR-0209 §3 の「出してよい項目」):\n%s", want, logs)
		}
	}
	// DB のエラー文そのものはクライアントに出さないが、ログには残してよい(原因追跡のため)。
	if !strings.Contains(logs, store.ErrUnavailable.Error()) {
		t.Errorf("ログに DB のエラーの手がかりが無い:\n%s", logs)
	}
}
