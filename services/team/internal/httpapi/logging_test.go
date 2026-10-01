package httpapi

// ログ規約(ADR-0209 §3・coding-rules §1)の受け入れテスト。AC-L1。
//
// 出してよい: 端末 ID・セッション ID・操作名・件数・所要時間・エラーコード。
// 出してはいけない: 個体の中身(種族・技・持ち物・特性・性格・SP)、ダメージの数値、
// **構築名・ポケモンのニックネーム**、リクエスト / レスポンスの本文そのもの。
//
// team-svc は構築名とニックネームという「利用者の自由入力」を扱う唯一のサービスなので、
// ここが ADR-0209 §3 のログ規約の要になる(record-svc より漏らしやすい)。

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/team/internal/store"
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
func TestLogsDoNotLeakTeamContents(t *testing.T) {
	const (
		forbiddenTeamName = "秘密の構築名"
		forbiddenNickname = "テストニックネーム"
	)

	m := memberJSON()
	m["speciesKey"] = speciesLeaf
	m["nickname"] = forbiddenNickname
	m["itemId"] = itemID
	m["abilityId"] = abilityID
	m["moveIds"] = []string{moveIDA, moveIDB}
	reqBody := body(t, teamJSON(forbiddenTeamName, m))

	buf := captureLogs(t)
	st := newFakeStore()
	h := NewHandler(st)

	created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA), reqBody), http.StatusCreated)
	serve(t, h, http.MethodGet, pathTeams, headers(deviceA), nil)
	serve(t, h, http.MethodGet, teamPath(created.Id), headers(deviceA), nil)
	serve(t, h, http.MethodPut, teamPath(created.Id), headers(deviceA), reqBody)
	serve(t, h, http.MethodDelete, teamPath(created.Id), headers(deviceA), nil)
	serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil)

	// 失敗の経路(入力検証・404・DB 到達不能)も見る。
	serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON(strings.Repeat(forbiddenTeamName, 10))))
	serve(t, h, http.MethodGet, teamPath(created.Id), headers(deviceA), nil)
	st.unavailable = true
	serve(t, h, http.MethodPost, pathTeams, headers(deviceA), reqBody)

	logs := buf.String()
	forbidden := []string{
		forbiddenTeamName, forbiddenNickname, speciesLeaf, itemID, abilityID, moveIDA, moveIDB,
		natureID, `"nickname"`,
	}
	for _, f := range forbidden {
		if strings.Contains(logs, f) {
			t.Errorf("ログに出してはいけない値 %q が含まれている(ADR-0209 §3):\n%s", f, logs)
		}
	}
}

// 出してよい項目は出ていること(ログを空にして AC-L1 を通す抜け道を塞ぐ)。
func TestLogsKeepAllowedFields(t *testing.T) {
	st := newFakeStore()
	st.unavailable = true
	buf := captureLogs(t)

	serve(t, NewHandler(st), http.MethodGet, pathTeams, headers(deviceA), nil)

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
