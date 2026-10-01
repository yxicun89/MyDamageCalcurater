package httpapi_test

// GET /readyz(readiness。issue #107・ADR-0129 §1)のテスト。/healthz(liveness。DB に触れない)とは別に、
// サービス提供に要る最小条件(DB に届く・必要なテーブルがある・data_versions・types・natures・既定の
// レギュレーションがある)を、短い締め切りの中で軽いクエリだけで確かめる。
// /readyz と /healthz は運用エンドポイントで、api/openapi.yaml の契約に含めない(/healthz と同じ扱い。ADR-0105 §1)。

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/pokedex/internal/httpapi"
	"example.com/pokecalc/services/pokedex/internal/storetest"
)

const readyzPath = "/readyz"

// readinessReads は /readyz が確かめる最小条件のクエリ(ADR-0129 §1)。
var readinessReads = []string{"ListDataVersions", "ListTypes", "ListNatures", "GetDefaultRegulation"}

// readinessAllowed は /readyz が呼んでよいメソッド(最小条件のクエリと、それを1つの読み取り専用 Tx で
// 読む場合の BeginTx / Commit / Rollback)。マスタ全体(種族・技・持ち物など)を読む重いクエリは呼ばない。
var readinessAllowed = map[string]bool{
	"ListDataVersions": true, "ListTypes": true, "ListNatures": true, "GetDefaultRegulation": true,
	storetest.MethodBeginTx: true, storetest.MethodCommit: true, storetest.MethodRollback: true,
}

// AC-R1(#107): マスタが揃っていれば /readyz は 200 {"status":"ok"}。端末ID/セッションID を要求しない。
// 最小条件の4クエリを全て呼び、それ以外の重いクエリは呼ばない。開いたままのトランザクションを残さない。
func TestReadyzOKWhenMasterIsComplete(t *testing.T) {
	q := storetest.New()
	h := newHandler(t, q)
	rec := do(t, h, http.MethodGet, readyzPath, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("/readyz = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Errorf("/readyz の本文 = %s", got)
	}
	called := map[string]bool{}
	for _, c := range q.Calls {
		called[c.Method] = true
		if !readinessAllowed[c.Method] {
			t.Errorf("/readyz が最小条件以外のクエリ %s を呼んだ(probe の周期で重い処理をしない)", c.Method)
		}
	}
	for _, m := range readinessReads {
		if !called[m] {
			t.Errorf("/readyz が %s を呼んでいない(必須集合の確認が足りない)", m)
		}
	}
	if open := q.OpenTxCount(); open != 0 {
		t.Errorf("/readyz の後に開いたままのトランザクションが %d 個ある", open)
	}
}

// AC-R2(#107): DB 未接続・migration 未実施(テーブルが無い = クエリの失敗)・マスタ未投入・必須集合の欠けでは
// /readyz は 503 master_unavailable(Error 形式)。DB の内部エラーの文言を本文に出さない。
func TestReadyzUnavailable(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(q *storetest.Querier)
	}{
		{"DB に接続できない", func(q *storetest.Querier) { q.Err = storetest.ErrDB }},
		{"data_versions が空(未投入)", func(q *storetest.Querier) { q.DataVersions = nil }},
		{"types が空", func(q *storetest.Querier) { q.Types = nil }},
		{"natures が空", func(q *storetest.Querier) { q.Natures = nil }},
		{"既定のレギュレーションが無い", func(q *storetest.Querier) { q.DefaultRegulation = nil }},
		{"data_versions のテーブルが無い(migration 未実施)", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListDataVersions": storetest.ErrDB}
		}},
		{"types のクエリが失敗", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListTypes": storetest.ErrDB}
		}},
		{"natures のテーブルが無い(000006 の migration 未実施)", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"ListNatures": storetest.ErrDB}
		}},
		{"既定のレギュレーションのクエリが失敗", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{"GetDefaultRegulation": storetest.ErrDB}
		}},
		{"トランザクションを開けない", func(q *storetest.Querier) {
			q.ErrByMethod = map[string]error{storetest.MethodBeginTx: storetest.ErrDB}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := storetest.New()
			tt.mutate(q)
			h := newHandler(t, q)
			rec := do(t, h, http.MethodGet, readyzPath, false)
			assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
			if strings.Contains(rec.Body.String(), storetest.ErrDB.Error()) {
				t.Errorf("DB の内部エラーの文言を応答に出している: %s", rec.Body.String())
			}
			if open := q.OpenTxCount(); open != 0 {
				t.Errorf("失敗の後に開いたままのトランザクションが %d 個ある", open)
			}
		})
	}
}

// AC-R3(#107): import が終われば、プロセスを再起動せずに(同じハンドラのまま)/readyz が 503 から 200 に変わる。
// DB が戻れば 200 に戻る(結果をキャッシュしない)。
func TestReadyzRecoversWithoutRestart(t *testing.T) {
	q := storetest.New()
	complete := storetest.New()
	q.DataVersions = nil
	h := newHandler(t, q)

	assertError(t, do(t, h, http.MethodGet, readyzPath, false), http.StatusServiceUnavailable, api.MasterUnavailable)

	q.DataVersions = complete.DataVersions // import が終わった
	if rec := do(t, h, http.MethodGet, readyzPath, false); rec.Code != http.StatusOK {
		t.Fatalf("import 後の /readyz = %d, want 200(再起動なしで Ready に戻る)\nbody=%s", rec.Code, rec.Body.String())
	}

	q.Err = storetest.ErrDB // DB が落ちた
	assertError(t, do(t, h, http.MethodGet, readyzPath, false), http.StatusServiceUnavailable, api.MasterUnavailable)
	if rec := do(t, h, http.MethodGet, "/healthz", false); rec.Code != http.StatusOK {
		t.Errorf("DB が落ちている間の /healthz = %d, want 200(liveness は DB に連動させない)", rec.Code)
	}

	q.Err = nil // DB が戻った
	if rec := do(t, h, http.MethodGet, readyzPath, false); rec.Code != http.StatusOK {
		t.Fatalf("DB 復旧後の /readyz = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
}

// AC-R4(#107・#323): DB が固まったとき、/readyz は readiness の締め切り(+ 許容)以内に 503 を返す
// (kubelet の probe の timeoutSeconds より先に、503 で NotReady を伝える)。DB には締め切り付きの context が渡る。
func TestReadyzReturnsUnavailableWithinReadinessDeadline(t *testing.T) {
	db := newStuckDB()
	// 要求の締め切りは長いままにして、/readyz が readiness の締め切りの方を使うことを確かめる。
	h := httpapi.NewHandler(db, httpapi.WithRequestTimeout(stuckCap), httpapi.WithReadinessTimeout(testDeadline))

	start := time.Now()
	rec := do(t, h, http.MethodGet, readyzPath, false)
	elapsed := time.Since(start)

	if elapsed > testDeadline+deadlineSlack {
		t.Fatalf("/readyz が %v かかった(readiness の締め切り %v + 許容 %v を超えた)", elapsed, testDeadline, deadlineSlack)
	}
	assertError(t, rec, http.StatusServiceUnavailable, api.MasterUnavailable)
	calls := db.calls()
	if len(calls) == 0 {
		t.Fatal("/readyz が DB を呼んでいない(DB に連動していない)")
	}
	for i, d := range calls {
		if d.IsZero() {
			t.Errorf("/readyz の DB の呼び出し %d に締め切りの無い context が渡った", i)
		}
	}
}

// AC-R5(#107): 本番の組み立て(オプション無し)でも、/readyz は DefaultReadinessTimeout の締め切りで DB を呼ぶ。
// readiness の締め切りは要求の締め切り以下(probe は軽く・早く判定する)。
func TestDefaultReadinessDeadlineIsApplied(t *testing.T) {
	if httpapi.DefaultReadinessTimeout <= 0 {
		t.Fatalf("DefaultReadinessTimeout = %v, want 正の値", httpapi.DefaultReadinessTimeout)
	}
	if httpapi.DefaultReadinessTimeout > httpapi.DefaultRequestTimeout {
		t.Errorf("DefaultReadinessTimeout(%v)が DefaultRequestTimeout(%v)より長い",
			httpapi.DefaultReadinessTimeout, httpapi.DefaultRequestTimeout)
	}
	db := &deadlineCapturingDB{Querier: storetest.New()}
	h := httpapi.NewHandler(db)
	start := time.Now()
	if rec := do(t, h, http.MethodGet, readyzPath, false); rec.Code != http.StatusOK {
		t.Fatalf("/readyz = %d, want 200\nbody=%s", rec.Code, rec.Body.String())
	}
	calls := db.captured()
	if len(calls) == 0 {
		t.Fatal("/readyz の DB の呼び出しが記録されていない(検査が空振りしている)")
	}
	for i, d := range calls {
		if d.IsZero() {
			t.Fatalf("/readyz の DB の呼び出し %d に締め切りの無い context が渡った", i)
		}
		if limit := start.Add(httpapi.DefaultReadinessTimeout + deadlineSlack); d.After(limit) {
			t.Errorf("/readyz の DB の呼び出し %d の締め切り %v が DefaultReadinessTimeout(%v)より遠い",
				i, d.Sub(start), httpapi.DefaultReadinessTimeout)
		}
	}
}

// AC-R6: /readyz は GET だけ(他のメソッドは 404 not_found。Error 形式)。/readyz と /healthz は契約外の
// 運用エンドポイントのまま(api/openapi.yaml に載せない。/healthz と同じ扱い)。
func TestReadyzMethodsAndContract(t *testing.T) {
	h := newHandler(t, storetest.New())
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		assertError(t, do(t, h, m, readyzPath, false), http.StatusNotFound, api.NotFound)
	}
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("契約を読めない: %v", err)
	}
	for _, p := range []string{readyzPath, "/healthz"} {
		if doc.Paths.Find(p) != nil {
			t.Errorf("契約に %s がある(運用エンドポイントは契約外。ADR-0105 §1・ADR-0129)", p)
		}
	}
}
