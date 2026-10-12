package httpapi

// GET /api/record/calc-history(listCalcHistory)の受け入れテスト(requirements.md §2「計算結果の自動保存」・
// ADR-0209 §3「calc_events の目的: 履歴表示」・ADR-0230)。AC-H1〜AC-H12 は ADR-0230「受け入れ条件」と同じ番号。

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/internal/calcevents"
)

// AC-H1: 1件の計算(operation = calc)だけを、計算した時刻の新しい順に返す。一括計算・逆算は載らない。
func TestCalcHistoryListsSingleCalcsNewestFirst(t *testing.T) {
	st := newFakeStore()
	base := historyNow()
	st.addCalc(t, deviceA, "e1", base.Add(-3*time.Hour), "move-old")
	st.addEvent(deviceA, "e2", calcevents.OperationBulk, base.Add(-2*time.Hour), envelopePayload(t, deviceA, calcevents.OperationBulk, base.Add(-2*time.Hour)))
	st.addCalc(t, deviceA, "e3", base.Add(-time.Hour), "move-new")
	st.addEvent(deviceA, "e4", calcevents.OperationReverse, base.Add(-30*time.Minute), envelopePayload(t, deviceA, calcevents.OperationReverse, base.Add(-30*time.Minute)))
	st.addCalc(t, deviceA, "e5", base.Add(-2*time.Hour-time.Minute), "move-mid")

	got := decodeHistory(t, serve(t, historyHandler(st), http.MethodGet, pathCalcHistory, headers(deviceA), nil))

	want := []string{"move-new", "move-mid", "move-old"}
	if g := moveIDs(got); strings.Join(g, ",") != strings.Join(want, ",") {
		t.Fatalf("行の並び = %v, want %v(calc だけ・新しい順)", g, want)
	}
	if !got.Items[0].OccurredAt.Equal(base.Add(-time.Hour)) {
		t.Errorf("occurredAt = %v, want %v(イベントの occurred_at そのもの)", got.Items[0].OccurredAt, base.Add(-time.Hour))
	}
	if got.Items[0].Result.MinPercent != 12.3 || got.Items[0].Result.MaxPercent != 45.6 {
		t.Errorf("result = %+v, want min 12.3 / max 45.6(イベントの minPercent / maxPercent をそのまま)", got.Items[0].Result)
	}
	if got.Items[0].Calc.Defender.SpeciesKey != speciesLeaf || got.Items[0].Calc.Attacker.SpeciesKey != speciesGuard {
		t.Errorf("calc の攻撃側・防御側 = %q / %q, want %q / %q",
			got.Items[0].Calc.Attacker.SpeciesKey, got.Items[0].Calc.Defender.SpeciesKey, speciesGuard, speciesLeaf)
	}
	if got.NextCursor != nil {
		t.Errorf("nextCursor = %q, want null(全件が1ページに収まる)", *got.NextCursor)
	}
}

// AC-H2: calc は Favorite.calc と同じ正規化(既定値を補った CalcRequest)。同じ計算の入力でお気に入りを作ったときの
// calc と JSON として一致する(Web・iOS が「お気に入りから計算を出す」処理をそのまま使える。ADR-0228)。
func TestCalcHistoryCalcIsNormalizedLikeFavorite(t *testing.T) {
	for _, tc := range []struct {
		name string
		calc map[string]any
	}{
		{"最小の入力(既定値を補う)", calcInput()},
		{"任意項目まで埋めた入力(落とさない)", fullCalcInput()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			h := historyHandler(st)
			at := historyNow().Add(-time.Minute)
			st.addEvent(deviceA, "e1", calcevents.OperationCalc, at, calcEventPayload(t, deviceA, at, tc.calc, 10, 20))

			rec := serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceA), nil)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
			}
			var raw struct {
				Items []struct {
					Calc json.RawMessage `json:"calc"`
				} `json:"items"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil || len(raw.Items) != 1 {
				t.Fatalf("1行の履歴として読めない: %v; body=%s", err, rec.Body.String())
			}

			fav := createFavoriteRaw(t, h, deviceA, favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), tc.calc), http.StatusCreated)
			if fav.Calc == nil {
				t.Fatal("お気に入りの calc が無い")
			}
			favCalc, err := json.Marshal(fav.Calc)
			if err != nil {
				t.Fatal(err)
			}
			if !jsonSemanticallyEqual(t, raw.Items[0].Calc, favCalc) {
				t.Errorf("履歴の calc がお気に入りの calc と同じ正規化になっていない:\n history=%s\nfavorite=%s", raw.Items[0].Calc, favCalc)
			}
		})
	}
}

// AC-H3: 返す項目はホワイトリスト(occurredAt・calc・result、result は minPercent・maxPercent)だけ。
// 端末 ID・セッション ID・schemaVersion・viaRecommendation・イベント ID・payload の未知のキーを返さない(ADR-0209 §3・ADR-0230 §2)。
func TestCalcHistoryReturnsOnlyWhitelistedFields(t *testing.T) {
	st := newFakeStore()
	at := historyNow().Add(-time.Minute)
	payload := calcEventPayload(t, deviceA, at, calcInput(), 10, 20)
	// 将来の版で増えうるキー(calc-svc 側の後方互換の追加)を、トップレベルと detail の中に足しておく。
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		t.Fatal(err)
	}
	m["futureEnvelopeField"] = "secret-envelope"
	m["detail"].(map[string]any)["futureDetailField"] = "secret-detail"
	payload, _ = json.Marshal(m)
	const eventID = "calc-events-987654"
	st.addEvent(deviceA, eventID, calcevents.OperationCalc, at, payload)

	rec := serve(t, historyHandler(st), http.MethodGet, pathCalcHistory, headers(deviceA), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200(未知のキーがあっても読む); body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	top := rawKeys(t, rec.Body.Bytes())
	assertKeySet(t, "ページ", top, "items", "nextCursor")
	var page struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || len(page.Items) != 1 {
		t.Fatalf("1行の履歴として読めない: %v; body=%s", err, body)
	}
	item := rawKeys(t, page.Items[0])
	assertKeySet(t, "行", item, "occurredAt", "calc", "result")
	assertKeySet(t, "result", rawKeys(t, item["result"]), "minPercent", "maxPercent")
	assertKeySet(t, "calc", rawKeys(t, item["calc"]), "format", "attacker", "defender", "moveId", "field", "options")

	for _, forbidden := range []string{
		deviceA, sessionID, eventID, "987654", "deviceId", "sessionId", "schemaVersion", "viaRecommendation",
		"detail", "operation", "secret-envelope", "secret-detail", "futureEnvelopeField", "futureDetailField",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("応答に返してはいけない値 %q が含まれている(ADR-0230 §2 のホワイトリスト):\n%s", forbidden, body)
		}
	}
}

func assertKeySet(t *testing.T, what string, got map[string]json.RawMessage, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s のキー = %v, want ちょうど %v", what, keysOf(got), want)
		return
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("%s にキー %q が無い(got %v)", what, k, keysOf(got))
		}
	}
}

// AC-H4: ページング。nextCursor をたどると全件を重複・欠落なく1回ずつ得て、最後のページは null。
// 同時刻の行(event_id だけが違う)をまたいでも重複・欠落しない(カーソルが時刻とイベントの両方を持つ)。
func TestCalcHistoryPagination(t *testing.T) {
	cases := []struct {
		name      string
		n         int
		limit     int
		wantPages int
	}{
		{"端数あり(5件を2件ずつ)", 5, 2, 3},
		{"ちょうど割り切れる(4件を2件ずつ。空のページを余計に返さない)", 4, 2, 2},
		{"1ページに収まる", 3, 20, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			base := historyNow().Add(-time.Hour)
			var want []string
			for i := tc.n - 1; i >= 0; i-- {
				// 2件ずつ同じ時刻にする(e0,e1 が同時刻・e2,e3 が同時刻 …)。
				at := base.Add(time.Duration(i/2) * time.Minute)
				st.addCalc(t, deviceA, "e"+string(rune('0'+i)), at, "m"+string(rune('0'+i)))
			}
			for i := tc.n - 1; i >= 0; i-- {
				want = append(want, "m"+string(rune('0'+i))) // 新しい順(同時刻は event_id の降順)
			}

			h := historyHandler(st)
			var got []string
			var cursor *string
			pages := 0
			for {
				pages++
				if pages > tc.n+2 {
					t.Fatalf("ページが終わらない(nextCursor が null にならない)")
				}
				p := decodeHistory(t, serve(t, h, http.MethodGet, historyPageURL(tc.limit, cursor), headers(deviceA), nil))
				if len(p.Items) > tc.limit {
					t.Fatalf("1ページ %d 件, want limit(%d)以下", len(p.Items), tc.limit)
				}
				if len(p.Items) == 0 {
					t.Fatalf("%d ページ目が空(最後のページで nextCursor を null にし、空のページを返さない)", pages)
				}
				got = append(got, moveIDs(p)...)
				if p.NextCursor == nil {
					break
				}
				if *p.NextCursor == "" {
					t.Fatal("nextCursor が空文字(続きが無ければ null)")
				}
				cursor = p.NextCursor
			}
			if pages != tc.wantPages {
				t.Errorf("ページ数 = %d, want %d", pages, tc.wantPages)
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("たどった行 = %v, want %v(重複・欠落なし・新しい順)", got, want)
			}
		})
	}
}

// AC-H4: ページの間に新しい計算が保存されても、2ページ目は1ページ目の続きから(先頭がずれて重複しない)。
func TestCalcHistoryCursorIsStableAgainstNewEvents(t *testing.T) {
	st := newFakeStore()
	base := historyNow().Add(-time.Hour)
	for i := 0; i < 4; i++ {
		st.addCalc(t, deviceA, "e"+string(rune('0'+i)), base.Add(time.Duration(i)*time.Minute), "m"+string(rune('0'+i)))
	}
	h := historyHandler(st)
	p1 := decodeHistory(t, serve(t, h, http.MethodGet, historyPageURL(2, nil), headers(deviceA), nil))
	if strings.Join(moveIDs(p1), ",") != "m3,m2" || p1.NextCursor == nil {
		t.Fatalf("1ページ目 = %v / cursor=%v, want [m3 m2] と続きあり", moveIDs(p1), p1.NextCursor)
	}
	st.addCalc(t, deviceA, "e9", base.Add(30*time.Minute), "m-newer")

	p2 := decodeHistory(t, serve(t, h, http.MethodGet, historyPageURL(2, p1.NextCursor), headers(deviceA), nil))
	if strings.Join(moveIDs(p2), ",") != "m1,m0" {
		t.Errorf("2ページ目 = %v, want [m1 m0](新しい行が増えても続きから)", moveIDs(p2))
	}
	if p2.NextCursor != nil {
		t.Errorf("2ページ目の nextCursor = %q, want null", *p2.NextCursor)
	}
}

// AC-H5: limit の既定は 20・範囲は 1〜50。範囲外・整数でない値は 400 invalid_input(listFrequentOpponents と同じ流儀。ADR-0208)。
func TestCalcHistoryLimit(t *testing.T) {
	st := newFakeStore()
	base := historyNow().Add(-time.Hour)
	for i := 0; i < 60; i++ {
		st.addCalc(t, deviceA, "e"+strings.Repeat("x", i), base.Add(time.Duration(i)*time.Second), "m")
	}
	h := historyHandler(st)

	t.Run("既定は20件で続きあり", func(t *testing.T) {
		p := decodeHistory(t, serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceA), nil))
		if len(p.Items) != 20 || p.NextCursor == nil {
			t.Errorf("件数 = %d・続き = %v, want 20 件・続きあり", len(p.Items), p.NextCursor != nil)
		}
	})
	for _, tc := range []struct {
		q    string
		want int
	}{{"1", 1}, {"50", 50}} {
		t.Run("有効な境界 limit="+tc.q, func(t *testing.T) {
			p := decodeHistory(t, serve(t, h, http.MethodGet, pathCalcHistory+"?limit="+tc.q, headers(deviceA), nil))
			if len(p.Items) != tc.want {
				t.Errorf("件数 = %d, want %d", len(p.Items), tc.want)
			}
		})
	}
	for _, bad := range []string{"0", "51", "-1", "abc", "1.5"} {
		t.Run("範囲外 limit="+bad, func(t *testing.T) {
			rec := serve(t, h, http.MethodGet, pathCalcHistory+"?limit="+bad, headers(deviceA), nil)
			assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
		})
	}
}

// AC-H6: 読めないカーソルは 400 invalid_input(store を呼ぶ前に弾く。500 にしない)。
func TestCalcHistoryRejectsBadCursor(t *testing.T) {
	bad := []struct{ name, q string }{
		{"空", ""},
		{"許されない文字", "%40%40%40"},
		{"base64 として読めるが中身が違う", base64.RawURLEncoding.EncodeToString([]byte("hello"))},
		{"base64 として読めない", "a"},
		{"長すぎる", strings.Repeat("A", 201)},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			st.addCalc(t, deviceA, "e1", historyNow().Add(-time.Minute), "m")
			rec := serve(t, historyHandler(st), http.MethodGet, pathCalcHistory+"?cursor="+tc.q, headers(deviceA), nil)
			assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
			for _, c := range st.calls {
				if c.method == "ListCalcHistory" {
					t.Error("読めないカーソルで store を呼んだ(先に 400 にする)")
				}
			}
		})
	}
}

// AC-H7: 端末分離(ADR-0209 §6)。他端末の行は出ず、store は自端末の ID でだけ呼ばれる。
// 記録の無い端末は items が空配列(null ではない)・nextCursor は null。
// 他端末で得たカーソルを渡しても、その端末の行は返らない(カーソルは位置だけを持つ)。
func TestCalcHistoryIsScopedToTheDevice(t *testing.T) {
	st := newFakeStore()
	base := historyNow().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		st.addCalc(t, deviceA, "a"+string(rune('0'+i)), base.Add(time.Duration(i)*time.Minute), "move-of-A")
	}
	st.addCalc(t, deviceB, "b0", base.Add(-time.Minute), "move-of-B")
	h := historyHandler(st)

	gotB := decodeHistory(t, serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceB), nil))
	if strings.Join(moveIDs(gotB), ",") != "move-of-B" {
		t.Errorf("端末 B の履歴 = %v, want [move-of-B](A の行が混ざらない)", moveIDs(gotB))
	}
	for _, id := range st.deviceIDsTouched() {
		if id != deviceB {
			t.Errorf("store が端末 %q で呼ばれた, want %q だけ(ADR-0209 §6-1)", id, deviceB)
		}
	}

	// A のカーソル(A の1件目の位置)を B が使っても、A の行は出ない。
	pA := decodeHistory(t, serve(t, h, http.MethodGet, historyPageURL(1, nil), headers(deviceA), nil))
	if pA.NextCursor == nil {
		t.Fatal("端末 A の1ページ目に続きが無い")
	}
	gotB2 := decodeHistory(t, serve(t, h, http.MethodGet, historyPageURL(10, pA.NextCursor), headers(deviceB), nil))
	for _, m := range moveIDs(gotB2) {
		if m == "move-of-A" {
			t.Errorf("他端末のカーソルで端末 A の行が返った(ADR-0209 §6-5)")
		}
	}

	// 記録の無い端末。
	const deviceC = "00000000-0000-4000-8000-00000000000c"
	rec := serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceC), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("記録の無い端末 = %d, want 200(404 にしない); body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"items":[]`) || !strings.Contains(rec.Body.String(), `"nextCursor":null`) {
		t.Errorf("記録の無い端末の本文 = %s, want items が [] で nextCursor が null", rec.Body.String())
	}
}

// AC-H7: 端末 ID をクエリ・ボディで受け取らない(400 unknown_field。ADR-0209 §2・§6-4)。ヘッダが無ければ 400 missing_header。
func TestCalcHistoryDeviceIDOnlyFromHeader(t *testing.T) {
	tests := []struct {
		name string
		path string
		body []byte
	}{
		{"クエリに deviceId", pathCalcHistory + "?deviceId=" + deviceB, nil},
		{"クエリに device_id", pathCalcHistory + "?device_id=" + deviceB, nil},
		{"ボディに deviceId", pathCalcHistory, []byte(`{"deviceId":"` + deviceB + `"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			st.addCalc(t, deviceB, "b0", historyNow().Add(-time.Minute), "move-of-B")
			rec := serve(t, historyHandler(st), http.MethodGet, tt.path, headers(deviceA), tt.body)
			assertErrorBody(t, rec, http.StatusBadRequest, api.UnknownField)
			for _, id := range st.deviceIDsTouched() {
				if id == deviceB {
					t.Errorf("ヘッダ外の deviceId で端末 %q が参照された(ADR-0209 §6-4)", id)
				}
			}
		})
	}
	t.Run("X-Device-Id が無い", func(t *testing.T) {
		hdr := http.Header{}
		hdr.Set("X-Session-Id", sessionID)
		rec := serve(t, historyHandler(newFakeStore()), http.MethodGet, pathCalcHistory, hdr, nil)
		assertErrorBody(t, rec, http.StatusBadRequest, api.MissingHeader)
	})
}

// AC-H8: 保持期間(90日)を過ぎた行は、失効ジョブが消す前でも返さない。httpapi は store に
// Since = 現在時刻 − 保持期間 を渡す(境界の行〈ちょうど Since〉は返す。失効ジョブの `occurred_at < cutoff` と対)。
func TestCalcHistoryRespectsRetention(t *testing.T) {
	st := &recordingStore{fakeStore: newFakeStore()}
	now := historyNow()
	st.addCalc(t, deviceA, "old", now.Add(-historyRetention-time.Hour), "move-expired")
	st.addCalc(t, deviceA, "new", now.Add(-historyRetention+time.Hour), "move-kept")

	before := time.Now().UTC()
	p := decodeHistory(t, serve(t, historyHandler(st), http.MethodGet, pathCalcHistory, headers(deviceA), nil))
	after := time.Now().UTC()

	if strings.Join(moveIDs(p), ",") != "move-kept" {
		t.Errorf("履歴 = %v, want [move-kept](保持期間を過ぎた行は返さない)", moveIDs(p))
	}
	if len(st.queries) == 0 {
		t.Fatal("ListCalcHistory が呼ばれていない")
	}
	since := st.queries[0].Since
	lo, hi := before.Add(-historyRetention), after.Add(-historyRetention)
	if since.Before(lo.Add(-time.Millisecond)) || since.After(hi.Add(time.Millisecond)) {
		t.Errorf("Since = %v, want 現在時刻 − 90日([%v, %v])", since, lo, hi)
	}
}

// AC-H9: 全削除(DELETE /api/record/device-data)の後は空。1回で消しきれない(partial)途中でも、
// 削除より前の行は返さない(墓石 purged_at 以前の行を返さない。ADR-0209 §7)。削除の後に計算した行は返す。
func TestCalcHistoryAfterDeviceDataDeletion(t *testing.T) {
	for _, tc := range []struct {
		name       string
		purgeLimit int
		wantStatus api.DeletionStatus
	}{
		{"completed", 1000, api.Completed},
		{"partial の途中", 1, api.Partial},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			st.purgeLimit = tc.purgeLimit
			// fake の PurgeDevice は自分の時計(st.now。2026-09-25)を墓石にするので、行もその前に置く。
			for i := 0; i < 3; i++ {
				st.addCalc(t, deviceA, "e"+string(rune('0'+i)), st.now.Add(-time.Duration(i+1)*time.Minute), "before-purge")
			}
			h := NewHandler(st) // 保持期間の下限を付けない(fake の時計が実時刻より古いため)
			if res := decodeDeletion(t, serve(t, h, http.MethodDelete, pathDeviceData, headers(deviceA), nil)); res.Status != tc.wantStatus {
				t.Fatalf("削除の status = %q, want %q", res.Status, tc.wantStatus)
			}
			p := decodeHistory(t, serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceA), nil))
			if len(p.Items) != 0 || p.NextCursor != nil {
				t.Errorf("削除後の履歴 = %v / cursor=%v, want 空(削除より前の行は返さない)", moveIDs(p), p.NextCursor)
			}

			st.addCalc(t, deviceA, "after", st.now.Add(time.Minute), "after-purge")
			p = decodeHistory(t, serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceA), nil))
			if strings.Join(moveIDs(p), ",") != "after-purge" {
				t.Errorf("削除後に計算した行 = %v, want [after-purge]", moveIDs(p))
			}
		})
	}
}

// AC-H10: 読むだけで記録を書き換えない(calc_events の行・内容は変わらない。失効の判定に効く値を進めない)。
// devices.last_seen_at の更新(TouchDevice)は他の操作と同じく行う(ADR-0209 §4。touchAndRun)。
func TestCalcHistoryIsReadOnlyAndTouchesDevice(t *testing.T) {
	st := newFakeStore()
	st.addCalc(t, deviceA, "e1", historyNow().Add(-time.Minute), "m")
	before := st.eventsOf(deviceA)

	decodeHistory(t, serve(t, historyHandler(st), http.MethodGet, pathCalcHistory, headers(deviceA), nil))

	after := st.eventsOf(deviceA)
	if len(after) != len(before) || after[0].EventID != before[0].EventID || !after[0].OccurredAt.Equal(before[0].OccurredAt) ||
		string(after[0].Payload) != string(before[0].Payload) {
		t.Errorf("読み取りで calc_events が変わった: before=%+v after=%+v", before, after)
	}
	var methods []string
	for _, c := range st.calls {
		methods = append(methods, c.method)
	}
	if strings.Join(methods, ",") != "TouchDevice,ListCalcHistory" {
		t.Errorf("store の呼び出し = %v, want [TouchDevice ListCalcHistory](書き込み系を呼ばない)", methods)
	}
}

// AC-H11: DB に届かなければ 503 store_unavailable。TouchDevice が失敗したら一覧は読まない(touchAndRun)。
func TestCalcHistoryStoreUnavailable(t *testing.T) {
	st := newFakeStore()
	st.unavailable = true
	rec := serve(t, historyHandler(st), http.MethodGet, pathCalcHistory, headers(deviceA), nil)
	assertErrorBody(t, rec, http.StatusServiceUnavailable, api.StoreUnavailable)
	for _, c := range st.calls {
		if c.method == "ListCalcHistory" {
			t.Error("TouchDevice が失敗したのに ListCalcHistory を呼んだ")
		}
	}
}

// AC-H12: 読めない行(壊れた JSON・Detail の無い calc・未知の schemaVersion・正規化できない入力)は飛ばして 200 を返す
// (1行の破損で履歴全体を見られなくしない)。飛ばした行もページの位置には数え、続きは正しくたどれる。
// ログには event_id だけを出し、payload の中身は出さない(ADR-0209 §3)。
func TestCalcHistorySkipsUnreadableRows(t *testing.T) {
	st := newFakeStore()
	base := historyNow().Add(-time.Hour)
	st.addCalc(t, deviceA, "e9", base.Add(9*time.Minute), "good-newest")
	st.addEvent(deviceA, "evt-broken-json", calcevents.OperationCalc, base.Add(8*time.Minute), []byte(`{"schemaVersion":1,"detail":{"moveId":"secret-broken"`))
	st.addEvent(deviceA, "e7", calcevents.OperationCalc, base.Add(7*time.Minute), envelopePayload(t, deviceA, calcevents.OperationCalc, base.Add(7*time.Minute)))
	v2 := calcEventPayload(t, deviceA, base.Add(6*time.Minute), calcInput(), 1, 2)
	v2 = []byte(strings.Replace(string(v2), `"schemaVersion":1`, `"schemaVersion":2`, 1))
	st.addEvent(deviceA, "e6", calcevents.OperationCalc, base.Add(6*time.Minute), v2)
	badSP := calcInput()
	badSP["attacker"] = map[string]any{"speciesKey": speciesGuard, "natureId": "fake-nature",
		"sp": map[string]any{"hp": 99, "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}}
	st.addEvent(deviceA, "e5", calcevents.OperationCalc, base.Add(5*time.Minute), calcEventPayload(t, deviceA, base.Add(5*time.Minute), badSP, 1, 2))
	st.addCalc(t, deviceA, "e1", base.Add(time.Minute), "good-oldest")

	buf := captureLogs(t)
	h := historyHandler(st)

	var got []string
	var cursor *string
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("ページが終わらない")
		}
		p := decodeHistory(t, serve(t, h, http.MethodGet, historyPageURL(2, cursor), headers(deviceA), nil))
		got = append(got, moveIDs(p)...)
		if p.NextCursor == nil {
			break
		}
		cursor = p.NextCursor
	}
	if strings.Join(got, ",") != "good-newest,good-oldest" {
		t.Errorf("たどった行 = %v, want [good-newest good-oldest](読めない行だけを飛ばす)", got)
	}
	logs := buf.String()
	if !strings.Contains(logs, "evt-broken-json") {
		t.Errorf("飛ばした行の event_id がログに無い(原因を追えない):\n%s", logs)
	}
	for _, forbidden := range []string{"secret-broken", speciesGuard, speciesLeaf, fakeMove} {
		if strings.Contains(logs, forbidden) {
			t.Errorf("ログに payload の中身 %q が出ている(ADR-0209 §3):\n%s", forbidden, logs)
		}
	}
}

// AC-L1(ADR-0209 §3)の履歴版: 成功・失敗のどちらでも、個体の中身・ダメージの数値・応答の本文をログに出さない。
func TestCalcHistoryLogsDoNotLeakContent(t *testing.T) {
	st := newFakeStore()
	at := historyNow().Add(-time.Minute)
	st.addEvent(deviceA, "e1", calcevents.OperationCalc, at, calcEventPayload(t, deviceA, at, fullCalcInput(), 87.6, 103.4))
	buf := captureLogs(t)
	h := historyHandler(st)

	serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceA), nil)
	serve(t, h, http.MethodGet, pathCalcHistory+"?cursor=a", headers(deviceA), nil)
	st.unavailable = true
	serve(t, h, http.MethodGet, pathCalcHistory, headers(deviceA), nil)

	logs := buf.String()
	for _, forbidden := range []string{speciesGuard, speciesLeaf, fakeMove, "87.6", "103.4", `"attacker"`} {
		if strings.Contains(logs, forbidden) {
			t.Errorf("ログに出してはいけない値 %q が含まれている(ADR-0209 §3):\n%s", forbidden, logs)
		}
	}
}
