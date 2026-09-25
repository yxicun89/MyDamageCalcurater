package httpapi

// 構築の CRUD(ADR-0213 §2・§3)の受け入れテスト。AC-T1〜AC-T7。
//
// team-svc は ID をマスタと照合しない(CLAUDE.md 絶対ルール4)。検証するのは
// 「マスタを引かなくても判断できること」だけ(名前の長さ・メンバー数・技の数と重複・SP の範囲と合計)。

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"example.com/pokecalc/services/internal/api"
	"example.com/pokecalc/services/team/internal/store"
)

// teamIDPattern は契約の TeamId(UUID の正準形)。
var teamIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func teamPath(id string) string { return pathTeams + "/" + id }

// AC-T1: 作成はサーバーが ID と時刻を決め、作った内容がそのまま取得できる。
func TestCreateTeamAssignsIDAndTimestamps(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams,
		headers(deviceA), body(t, teamJSON("雨パ", memberJSON()))), http.StatusCreated)

	if !teamIDPattern.MatchString(created.Id) {
		t.Errorf("id = %q は契約の TeamId(UUID)の形でない", created.Id)
	}
	if created.Name != "雨パ" {
		t.Errorf("name = %q, want %q", created.Name, "雨パ")
	}
	if len(created.Members) != 1 {
		t.Fatalf("members = %d 件, want 1", len(created.Members))
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Errorf("createdAt / updatedAt が空(%v / %v)", created.CreatedAt, created.UpdatedAt)
	}
	if !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Errorf("作成直後は createdAt == updatedAt であること(%v != %v)", created.CreatedAt, created.UpdatedAt)
	}

	got := decodeTeam(t, serve(t, h, http.MethodGet, teamPath(created.Id), headers(deviceA), nil), http.StatusOK)
	if got.Id != created.Id || got.Name != created.Name || len(got.Members) != 1 {
		t.Errorf("取得した構築 = %+v, want 作成したもの %+v", got, created)
	}
}

// AC-T1: `id` / `createdAt` / `updatedAt` はサーバーが決めるので、要求に入れたら 400 unknown_field
// (クライアントが ID を決められると、他端末の ID を狙って作る経路ができる)。
func TestCreateTeamRejectsServerOwnedFields(t *testing.T) {
	for _, field := range []string{"id", "createdAt", "updatedAt"} {
		t.Run(field, func(t *testing.T) {
			st := newFakeStore()
			req := teamJSON("雨パ", memberJSON())
			req[field] = "11111111-2222-4333-8444-555555555555"
			rec := serve(t, NewHandler(st), http.MethodPost, pathTeams, headers(deviceA), body(t, req))
			assertErrorBody(t, rec, http.StatusBadRequest, api.UnknownField)
		})
	}
}

// AC-T2: マスタを引かずに判断できる入力検証。すべて 400 invalid_input。
func TestCreateTeamValidatesInput(t *testing.T) {
	longName := strings.Repeat("あ", 51)
	longNickname := strings.Repeat("ね", 25)

	manyMembers := make([]map[string]any, 7)
	for i := range manyMembers {
		manyMembers[i] = memberJSON()
	}

	fiveMoves := memberJSON()
	fiveMoves["moveIds"] = []string{"m1", "m2", "m3", "m4", "m5"}

	dupMoves := memberJSON()
	dupMoves["moveIds"] = []string{moveIDA, moveIDA}

	spOver := memberJSON()
	spOver["sp"] = spJSON(33, 0, 0, 0, 0, 0)

	spNegative := memberJSON()
	spNegative["sp"] = spJSON(-1, 0, 0, 0, 0, 0)

	spTotalOver := memberJSON()
	spTotalOver["sp"] = spJSON(32, 32, 3, 0, 0, 0) // 合計 67 > 66

	nicknameTooLong := memberJSON()
	nicknameTooLong["nickname"] = longNickname

	// critic 指摘 重要2: DB の列幅(species_key VARCHAR(16)・item_id VARCHAR(64))を超える入力は
	// INSERT 失敗による 503 store_unavailable ではなく、ここで 400 invalid_input にすること。
	speciesKeyTooLong := memberJSON()
	speciesKeyTooLong["speciesKey"] = strings.Repeat("9", maxSpeciesKeyRunes+1)

	itemIDTooLong := memberJSON()
	itemIDTooLong["itemId"] = strings.Repeat("i", maxIDRunes+1)

	tests := []struct {
		name string
		req  map[string]any
	}{
		{"名前が空", teamJSON("")},
		{"名前が空白だけ", teamJSON("   ")},
		{"名前が長すぎる(51文字)", teamJSON(longName)},
		{"メンバーが7体", teamJSON("多すぎ", manyMembers...)},
		{"技が5つ", teamJSON("技多すぎ", fiveMoves)},
		{"同じ技の重複", teamJSON("技重複", dupMoves)},
		{"SP が1ステータスで33", teamJSON("SP過大", spOver)},
		{"SP が負", teamJSON("SP負", spNegative)},
		{"SP の合計が67", teamJSON("SP合計過大", spTotalOver)},
		{"ニックネームが25文字", teamJSON("ニックネーム長すぎ", nicknameTooLong)},
		{"speciesKeyが長すぎる(17文字)", teamJSON("speciesKey長すぎ", speciesKeyTooLong)},
		{"itemIdが長すぎる(65文字)", teamJSON("itemId長すぎ", itemIDTooLong)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathTeams, headers(deviceA), body(t, tt.req))
			assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
			if n := st.rowsLeft(deviceA); n != 0 {
				t.Errorf("検証に落ちた要求で %d 行保存された, want 0", n)
			}
		})
	}
}

// AC-T2 の境界(通る側): 上限ちょうどは通る。上限を「以上で弾く」実装にしないこと。
func TestCreateTeamAcceptsBoundaryValues(t *testing.T) {
	sixMembers := make([]map[string]any, 6)
	for i := range sixMembers {
		m := memberJSON()
		m["moveIds"] = []string{"m1", "m2", "m3", "m4"} // ちょうど4つ
		m["sp"] = spJSON(32, 32, 2, 0, 0, 0)            // 1ステータス上限32・合計ちょうど66
		sixMembers[i] = m
	}
	req := teamJSON(strings.Repeat("あ", 50), sixMembers...) // 名前ちょうど50文字

	st := newFakeStore()
	created := decodeTeam(t, serve(t, NewHandler(st), http.MethodPost, pathTeams,
		headers(deviceA), body(t, req)), http.StatusCreated)
	if len(created.Members) != 6 {
		t.Errorf("members = %d 件, want 6", len(created.Members))
	}
}

// 軽微3: 構築名ちょうど1文字・ニックネームちょうど24文字も通ること(落ちる側〈0文字・25文字〉は
// TestCreateTeamValidatesInput が固定済み)。
func TestCreateTeamAcceptsNameAndNicknameLowerBoundary(t *testing.T) {
	m := memberJSON()
	m["nickname"] = strings.Repeat("ね", maxNicknameRunes) // ちょうど24文字

	st := newFakeStore()
	created := decodeTeam(t, serve(t, NewHandler(st), http.MethodPost, pathTeams,
		headers(deviceA), body(t, teamJSON("あ", m))), http.StatusCreated) // 名前ちょうど1文字
	if created.Name != "あ" {
		t.Errorf("name = %q, want %q", created.Name, "あ")
	}
	if len(created.Members) != 1 || created.Members[0].Nickname == nil || utf8.RuneCountInString(*created.Members[0].Nickname) != maxNicknameRunes {
		t.Errorf("members = %+v, want ニックネームちょうど%d文字が通る", created.Members, maxNicknameRunes)
	}
}

// AC-T3(ADR-0213 §3): team-svc はマスタを引かない。実在しない ID でも保存できる
// (pokedex-svc の DB に触らない。CLAUDE.md 絶対ルール4。妥当性はクライアントと calc-svc が担保する)。
func TestCreateTeamDoesNotValidateAgainstMaster(t *testing.T) {
	m := memberJSON()
	m["speciesKey"] = "9999-999" // 形式は契約どおりだがマスタには無い
	m["natureId"] = "存在しない性格"
	m["itemId"] = "存在しない持ち物"
	m["abilityId"] = "存在しない特性"
	m["moveIds"] = []string{"存在しない技"}

	st := newFakeStore()
	rec := serve(t, NewHandler(st), http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON("架空", m)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201(マスタ照合はしない。ADR-0213 §3); body=%s", rec.Code, rec.Body.String())
	}
	for _, forbidden := range []api.ErrorCode{api.UnknownSpecies, api.UnknownMove, api.UnknownItem, api.UnknownAbility, api.UnknownNature} {
		if strings.Contains(rec.Body.String(), string(forbidden)) {
			t.Errorf("team-svc が %q を返した(マスタ照合は担当外)", forbidden)
		}
	}
}

// AC-T4: メンバーの任意項目が往復すること(ニックネーム・持ち物・特性・テラス・技の並び)。
func TestTeamMemberRoundTrip(t *testing.T) {
	m := memberJSON()
	m["speciesKey"] = speciesLeaf
	m["nickname"] = "はっぱくん"
	m["moveIds"] = []string{moveIDB, moveIDA} // 並び順も保つ
	m["itemId"] = itemID
	m["abilityId"] = abilityID
	m["teraType"] = "grass"
	m["sp"] = spJSON(4, 0, 0, 32, 0, 30)

	st := newFakeStore()
	h := NewHandler(st)
	created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams,
		headers(deviceA), body(t, teamJSON("葉っぱ", m))), http.StatusCreated)

	got := decodeTeam(t, serve(t, h, http.MethodGet, teamPath(created.Id), headers(deviceA), nil), http.StatusOK)
	if len(got.Members) != 1 {
		t.Fatalf("members = %d 件, want 1", len(got.Members))
	}
	mem := got.Members[0]
	if string(mem.SpeciesKey) != speciesLeaf {
		t.Errorf("speciesKey = %q, want %q", mem.SpeciesKey, speciesLeaf)
	}
	if mem.Nickname == nil || *mem.Nickname != "はっぱくん" {
		t.Errorf("nickname = %v, want はっぱくん", mem.Nickname)
	}
	if mem.MoveIds == nil || len(*mem.MoveIds) != 2 || (*mem.MoveIds)[0] != moveIDB || (*mem.MoveIds)[1] != moveIDA {
		t.Errorf("moveIds = %v, want [%s %s](並び順も保つ)", mem.MoveIds, moveIDB, moveIDA)
	}
	if mem.ItemId == nil || *mem.ItemId != itemID {
		t.Errorf("itemId = %v, want %q", mem.ItemId, itemID)
	}
	if mem.AbilityId == nil || *mem.AbilityId != abilityID {
		t.Errorf("abilityId = %v, want %q", mem.AbilityId, abilityID)
	}
	if mem.TeraType == nil || *mem.TeraType != api.PokeTypeGrass {
		t.Errorf("teraType = %v, want grass", mem.TeraType)
	}
	if mem.Sp.Spa != 32 || mem.Sp.Spe != 30 || mem.Sp.Hp != 4 {
		t.Errorf("sp = %+v, want hp=4 spa=32 spe=30", mem.Sp)
	}
}

// 空文字のニックネームは「未設定」として扱う(空文字と null を両方持たせない)。
func TestEmptyNicknameIsStoredAsUnset(t *testing.T) {
	m := memberJSON()
	m["nickname"] = ""

	st := newFakeStore()
	created := decodeTeam(t, serve(t, NewHandler(st), http.MethodPost, pathTeams,
		headers(deviceA), body(t, teamJSON("空名", m))), http.StatusCreated)
	if len(created.Members) != 1 {
		t.Fatalf("members = %d 件, want 1", len(created.Members))
	}
	if n := created.Members[0].Nickname; n != nil && *n != "" {
		t.Errorf("nickname = %q, want 未設定(null)", *n)
	}
}

// AC-T5: 1端末が持てる構築の上限(ADR-0213 §2)。上限に達した端末の作成は 400 invalid_input。
func TestCreateTeamRespectsPerDeviceLimit(t *testing.T) {
	st := newFakeStore()
	st.teamLimit = 2
	st.seed(deviceA, 2, 0)
	h := NewHandler(st)

	rec := serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON("3つ目")))
	assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)

	// 別端末は影響を受けない(上限は端末ごと)。
	if rec := serve(t, h, http.MethodPost, pathTeams, headers(deviceB), body(t, teamJSON("B の1つ目"))); rec.Code != http.StatusCreated {
		t.Errorf("端末 B の作成 status = %d, want 201(上限は端末ごと)", rec.Code)
	}
}

// AC-T6: 一覧は更新の新しい順で、記録が無ければ空配列(null にしない・404 にしない)。
func TestListTeamsOrderAndEmpty(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	empty := serve(t, h, http.MethodGet, pathTeams, headers(deviceA), nil)
	if got := strings.TrimSpace(empty.Body.String()); got != "[]" {
		t.Errorf("構築が無い端末の本文 = %q, want \"[]\"(null にしない)", got)
	}
	if len(decodeTeams(t, empty)) != 0 {
		t.Error("構築が無い端末の一覧が空でない")
	}

	first := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON("1つ目"))), http.StatusCreated)
	second := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON("2つ目"))), http.StatusCreated)

	list := decodeTeams(t, serve(t, h, http.MethodGet, pathTeams, headers(deviceA), nil))
	if len(list) != 2 {
		t.Fatalf("一覧 = %d 件, want 2", len(list))
	}
	if list[0].Id != second.Id || list[1].Id != first.Id {
		t.Errorf("一覧の並び = [%s %s], want 更新の新しい順 [%s %s]", list[0].Id, list[1].Id, second.Id, first.Id)
	}
}

// AC-T7: PUT は名前とメンバー全体を置き換える(部分更新ではない)。createdAt は変わらず updatedAt は進む。
func TestUpdateTeamReplacesWholeTeam(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams,
		headers(deviceA), body(t, teamJSON("旧名", memberJSON(), memberJSON()))), http.StatusCreated)

	updated := decodeTeam(t, serve(t, h, http.MethodPut, teamPath(created.Id),
		headers(deviceA), body(t, teamJSON("新名", memberJSON()))), http.StatusOK)

	if updated.Id != created.Id {
		t.Errorf("id = %q, want %q(置換で ID は変わらない)", updated.Id, created.Id)
	}
	if updated.Name != "新名" {
		t.Errorf("name = %q, want 新名", updated.Name)
	}
	if len(updated.Members) != 1 {
		t.Errorf("members = %d 件, want 1(2体 → 1体の置換)", len(updated.Members))
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("createdAt = %v, want %v(作成時刻は変わらない)", updated.CreatedAt, created.CreatedAt)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("updatedAt = %v は %v より後でなければならない(失効判定の基準。ADR-0209 §4)", updated.UpdatedAt, created.UpdatedAt)
	}
}

// members を省いた PUT は「メンバーなし」への置換(部分更新として無視しない)。
func TestUpdateTeamWithoutMembersClearsMembers(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams,
		headers(deviceA), body(t, teamJSON("旧名", memberJSON()))), http.StatusCreated)

	updated := decodeTeam(t, serve(t, h, http.MethodPut, teamPath(created.Id),
		headers(deviceA), body(t, map[string]any{"name": "名前だけ"})), http.StatusOK)
	if len(updated.Members) != 0 {
		t.Errorf("members = %d 件, want 0(省略は「メンバーなし」への置換)", len(updated.Members))
	}
}

// 存在しない ID への GET / PUT / DELETE は 404 not_found。
func TestUnknownTeamIDIsNotFound(t *testing.T) {
	const unknownID = "11111111-2222-4333-8444-999999999999"
	tests := []struct {
		method string
		reqBdy []byte
	}{
		{http.MethodGet, nil},
		{http.MethodPut, nil},
		{http.MethodDelete, nil},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			st := newFakeStore()
			st.seed(deviceA, 1, 1)
			var b []byte
			if tt.method == http.MethodPut {
				b = body(t, teamJSON("置換"))
			}
			rec := serve(t, NewHandler(st), tt.method, teamPath(unknownID), headers(deviceA), b)
			assertErrorBody(t, rec, http.StatusNotFound, api.NotFound)
		})
	}
}

// 軽微4: 形式が UUID でない teamId(下流は形式を検証しない。ADR-0209 §2 の
// DeviceId の docstring と同じ扱い)も 404 not_found になる(400 にしない)。
func TestMalformedTeamIDIsNotFound(t *testing.T) {
	tests := []struct {
		name   string
		method string
	}{
		{"GET", http.MethodGet},
		{"PUT", http.MethodPut},
		{"DELETE", http.MethodDelete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			st.seed(deviceA, 1, 1)
			var b []byte
			if tt.method == http.MethodPut {
				b = body(t, teamJSON("置換"))
			}
			rec := serve(t, NewHandler(st), tt.method, teamPath("abc"), headers(deviceA), b)
			assertErrorBody(t, rec, http.StatusNotFound, api.NotFound)
		})
	}
}

// 削除は 204(本文なし)。同じ構築をもう一度消すと 404(ADR-0213 §2)。
func TestDeleteTeamThenNotFound(t *testing.T) {
	st := newFakeStore()
	ids := st.seed(deviceA, 2, 3)
	h := NewHandler(st)

	rec := serve(t, h, http.MethodDelete, teamPath(ids[0]), headers(deviceA), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if b := strings.TrimSpace(rec.Body.String()); b != "" {
		t.Errorf("204 に本文がある: %q", b)
	}

	again := serve(t, h, http.MethodDelete, teamPath(ids[0]), headers(deviceA), nil)
	assertErrorBody(t, again, http.StatusNotFound, api.NotFound)

	// もう1件は残っている(1件の削除が他の構築を巻き込まない)。
	if list := decodeTeams(t, serve(t, h, http.MethodGet, pathTeams, headers(deviceA), nil)); len(list) != 1 {
		t.Errorf("残りの構築 = %d 件, want 1", len(list))
	}
}

// 壊れた JSON は 400 invalid_json、契約にないフィールドは 400 unknown_field。
func TestMalformedBodies(t *testing.T) {
	tests := []struct {
		name     string
		reqBdy   []byte
		wantCode api.ErrorCode
	}{
		{"JSON として壊れている", []byte(`{`), api.InvalidJson},
		{"本文が空", nil, api.InvalidJson},
		{"契約にないフィールド", []byte(`{"name":"x","format":"single"}`), api.UnknownField},
		{"メンバーに契約にないフィールド", []byte(`{"name":"x","members":[{"speciesKey":"9002-000","natureId":"n","sp":{"hp":0,"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"level":50}]}`), api.UnknownField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathTeams, headers(deviceA), tt.reqBdy)
			assertErrorBody(t, rec, http.StatusBadRequest, tt.wantCode)
		})
	}
}

// DB に届かないときは、どの操作でも 503 store_unavailable(内部のエラー文を外に出さない)。
func TestOperationsReturnStoreUnavailable(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		reqBdy func(t *testing.T) []byte
	}{
		{"一覧", http.MethodGet, pathTeams, nil},
		{"作成", http.MethodPost, pathTeams, func(t *testing.T) []byte { return body(t, teamJSON("x")) }},
		{"取得", http.MethodGet, teamPath(fakeTeamID(1)), nil},
		{"更新", http.MethodPut, teamPath(fakeTeamID(1)), func(t *testing.T) []byte { return body(t, teamJSON("x")) }},
		{"削除", http.MethodDelete, teamPath(fakeTeamID(1)), nil},
		{"全削除", http.MethodDelete, pathDeviceData, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			st.unavailable = true
			var b []byte
			if tt.reqBdy != nil {
				b = tt.reqBdy(t)
			}
			rec := serve(t, NewHandler(st), tt.method, tt.path, headers(deviceA), b)
			assertErrorBody(t, rec, http.StatusServiceUnavailable, api.StoreUnavailable)
			if s := rec.Body.String(); strings.Contains(s, "fake:") || strings.Contains(s, store.ErrUnavailable.Error()) {
				t.Errorf("応答に内部のエラー文が漏れている: %s", s)
			}
		})
	}
}
