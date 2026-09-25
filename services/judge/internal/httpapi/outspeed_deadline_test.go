package httpapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"example.com/pokecalc/services/judge/internal/api"
)

// issue #213: 上流(pokedex-svc / calc-svc)が遅くても、judge は必ず JSON のエラー
// (503 upstream_unavailable)を返し、クライアントが空応答(HTTP 000 / Empty reply)を受け取らない。
// deps.RequestTimeout(ADR-0707 §2。JUDGE_REQUEST_TIMEOUT から main.go が渡す)が 1 リクエスト
// 全体の context.WithTimeout を張り、期限を超えたらまだ発行していない上流呼び出しを行わない。
//
// これらのテストはこのコミットの時点では失敗する(deps.RequestTimeout はまだ存在せず、
// outspeedAndKo はリクエスト全体の期限を持たない)。実装は implementer の担当(ADR-0707)。

// naiveFullSequenceCalls: defenders 6 件(ADR-0703 §1 の上限)を打ち切らずに全件完走したときの
// 上流呼び出し回数(natures 1 + species (1+6) + moves (1+6) + calc 2×6 = 27)。issue #213 の見積り
// 「2+4N=26」は natures の 1 回を含めていない数え方の差で、実質同じ規模感を指す。
const naiveFullSequenceCalls = 1 + (1 + 6) + (1 + 6) + 2*6

// deadlineCallBudget: 期限超過で打ち切られたと言える上限。naiveFullSequenceCalls よりずっと
// 小さいが、CI のタイミングのぶれ(1〜2 回多く進んでしまう程度)を吸収できるだけの余裕を持たせる。
const deadlineCallBudget = 15

// deadlineCandidateSpeciesKey は、このテスト専用の架空の種族キー(既存の
// attacker/defender/defender2/defender3SpeciesKey とは重ならない範囲。CLAUDE.md のドメイン規約:
// 実マスタを使わない)。
func deadlineCandidateSpeciesKey(i int) string {
	return fmt.Sprintf("95%02d-000", i)
}

// sixSlowDefenders は defenders の上限(ADR-0703 §1)いっぱいの 6 件。moveId は bodyWithDefenders
// が候補ごとに別の架空 ID を補う。
func sixSlowDefenders() []map[string]any {
	defenders := make([]map[string]any, 0, 6)
	for i := 0; i < 6; i++ {
		defenders = append(defenders, individual(deadlineCandidateSpeciesKey(i), natureNeutralID, 0))
	}
	return defenders
}

// TestOutspeedAndKoOverallDeadline: 上流の各呼び出しが遅くても(perCallDelay。1 回ごとの
// タイムアウト JUDGE_UPSTREAM_TIMEOUT には引っかからない速さ)、judge は deps.RequestTimeout の
// 期限内に 503 upstream_unavailable の JSON を返し(issue #213: 空応答にならない)、期限超過の時点で
// まだ発行していない上流呼び出しを行わない。
func TestOutspeedAndKoOverallDeadline(t *testing.T) {
	t.Parallel()

	const (
		// 1 回ごとの遅さ。upstreams.delay が natures・species・moves・calc のすべてに一律にかかる。
		perCallDelay = 50 * time.Millisecond
		// リクエスト全体の期限(ADR-0707 §1 の JUDGE_REQUEST_TIMEOUT に相当)。natures(1) →
		// attacker species(1) → attacker move(1) → candidate[0] species(1) までは
		// 4 × 50ms = 200ms で完了し、candidate[0] move(5 回目)の途中で期限(220ms)が来る想定。
		requestTimeout = 220 * time.Millisecond
		// 全件(27 回)完走したときの所要時間(27 × 50ms = 1350ms)よりずっと短い上限で確かめる。
		// 実装後の実測はおおむね requestTimeout 前後になるはずだが、CI のぶれを見込んで緩めに取る。
		maxElapsed = 700 * time.Millisecond
	)

	stub := &upstreams{delay: perCallDelay}
	deps := newUpstreams(t, stub)
	deps.RequestTimeout = requestTimeout

	body := bodyWithDefenders(sixSlowDefenders()...)

	start := time.Now()
	recorder := postOutspeed(deps, body, nil)
	elapsed := time.Since(start)

	assertStatusAndCode(t, recorder, http.StatusServiceUnavailable, api.UpstreamUnavailable)

	if elapsed > maxElapsed {
		t.Errorf("応答まで %v かかった。deps.RequestTimeout(%v)の期限内に 503 を返すこと(issue #213)。"+
			"全件完走すると %d 回・数百ms以上かかる想定", elapsed, requestTimeout, naiveFullSequenceCalls)
	}

	natures, speciesKeys, calcCalls := stub.counts()
	moveKeys := stub.moveCalls()
	total := natures + len(speciesKeys) + len(moveKeys) + calcCalls
	if total >= deadlineCallBudget {
		t.Errorf("上流を合計 %d 回呼んでいる(natures=%d species=%d moves=%d calc=%d)。"+
			"期限超過の時点でまだ発行していない上流呼び出しを行わないこと(issue #213)。"+
			"打ち切らずに全件やり切ると %d 回になるはず(deadlineCallBudget=%d 未満で打ち切られたことを確かめる)",
			total, natures, len(speciesKeys), len(moveKeys), calcCalls, naiveFullSequenceCalls, deadlineCallBudget)
	}
	if calcCalls > 0 {
		t.Errorf("calc-svc を %d 回呼んでいる。requestTimeout は species/moves の途中で尽きる想定で、"+
			"calc-svc まで届かないはず(perCallDelay/requestTimeout の見積りが外れている可能性がある)", calcCalls)
	}
}

// TestOutspeedAndKoWithinDeadlineUnaffected: 上流が deps.RequestTimeout の期限内に十分速く
// 応答するときは、応答・上流の呼び出し回数のどちらも変わらない(ADR-0707 受け入れ条件1: 正常系不変)。
// upstreams.delay を設定しない(既定 0)ので、このテストは実装前後どちらでも緑になるはずの回帰確認。
func TestOutspeedAndKoWithinDeadlineUnaffected(t *testing.T) {
	t.Parallel()

	stub := &upstreams{}
	deps := newUpstreams(t, stub)
	deps.RequestTimeout = 200 * time.Millisecond

	recorder := postOutspeed(deps, validBody(), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	natures, speciesKeys, calcCalls := stub.counts()
	if natures != 1 || len(speciesKeys) != 2 || calcCalls != 2 {
		t.Errorf("上流の呼び出し回数 = natures:%d species:%d calc:%d, want 1/2/2(deps.RequestTimeout を"+
			"設定しても、期限内に終わる正常系の呼び出し回数は変わらない)", natures, len(speciesKeys), calcCalls)
	}
}
