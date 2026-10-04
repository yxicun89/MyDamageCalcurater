package httpapi

// お気に入りに計算の入力全体(`calc` = 契約の CalcRequest)を持たせる(ADR-0228。usability-round2 F-09
// 「クリックするとそのときのダメージ計算がすぐ出せる」)の受け入れテスト。AC-FC1〜AC-FC9。
//
// **test-first(ADR-0003)**: 実装前に書いた。前提にする形(実装者向け):
//
//   - 本文の受け口 favoriteRequest に `calc` を足す(DisallowUnknownFields は calc の中にも効かせる)。
//   - 正規化(ADR-0228 §2): calc の attacker / defender は individual と同じ正規化(同じ関数を使う)。format は必須
//     (欠落・未知は calc-svc と同じ 400 invalid_enum)、moveId は空でない文字列、field は weather・terrain・
//     attackerScreens・defenderScreens(各 reflect・lightScreen・auroraVeil)をすべて、options は critical を
//     すべて既定値で補う。
//   - 保存する snapshot は `{"label":…,"individual":{…},"calc":{…}}`。**calc が無いときは "calc" キーを出さない**
//     (従来とバイト列が同じ = 既存行の snapshot_hash・重複判定が変わらない)。calc の中のキー順は
//     format, attacker, defender, moveId, field{weather, terrain, attackerScreens, defenderScreens}, options{critical}。
//   - 正規化後の snapshot が 4096 バイトを超えたら 400 invalid_input(calc の有無にかかわらず。ADR-0228 §3)。
//   - 範囲の検証(SP 各0..32・合計66・ランク±6・level 50)は individual と calc で同じ実装を使う
//     (record-svc は calc-svc に依存しない。定数は engine の MaxSPPerStat・MaxSPTotal 等を参照してよい)。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
)

const fakeMove = "fake-move"

// calcInput は最小の計算入力(任意項目を送らない。既定値の補完を確かめるため)。
func calcInput() map[string]any {
	return map[string]any{
		"format":   "single",
		"attacker": minimalIndividual(speciesGuard),
		"defender": minimalIndividual(speciesLeaf),
		"moveId":   fakeMove,
	}
}

// fullCalcInput は任意項目まで埋めた計算入力(往復で落ちないことを確かめる)。
func fullCalcInput() map[string]any {
	return map[string]any{
		"format":   "double",
		"attacker": fullIndividual(speciesGuard),
		"defender": fullIndividual(speciesLeaf),
		"moveId":   fakeMove,
		"field": map[string]any{
			"weather":         "rain",
			"terrain":         "electric",
			"attackerScreens": map[string]any{"reflect": true},
			"defenderScreens": map[string]any{"lightScreen": true, "auroraVeil": true},
		},
		"options": map[string]any{"critical": true},
	}
}

func favoriteWithCalcBody(t *testing.T, label any, individual map[string]any, calc any) []byte {
	t.Helper()
	m := map[string]any{"individual": individual, "calc": calc}
	if label != nil {
		m["label"] = label
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// rawKeys は応答の1件を生の JSON で読む(キーの有無・null を見分けるため)。
func rawKeys(t *testing.T, b []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("JSON オブジェクトとして読めない: %v; %s", err, b)
	}
	return m
}

// 正規化した計算入力の期待(calcInput() を送ったとき)。キー順もこのとおり(snapshot の正規形)。
const (
	wantAttackerCanonical = `{"speciesKey":"9002-000","level":50,"natureId":"fake-nature","sp":{"hp":32,"atk":0,"def":2,"spa":0,"spd":0,"spe":32},` +
		`"ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"}`
	wantDefenderCanonical = `{"speciesKey":"9003-000","level":50,"natureId":"fake-nature","sp":{"hp":32,"atk":0,"def":2,"spa":0,"spd":0,"spe":32},` +
		`"ranks":{"atk":0,"def":0,"spa":0,"spd":0,"spe":0},"status":"none"}`
	wantCalcCanonical = `{"format":"single","attacker":` + wantAttackerCanonical + `,"defender":` + wantDefenderCanonical +
		`,"moveId":"fake-move","field":{"weather":"none","terrain":"none",` +
		`"attackerScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false},` +
		`"defenderScreens":{"reflect":false,"lightScreen":false,"auroraVeil":false}},"options":{"critical":false}}`
)

// --- AC-FC1: 往復 -------------------------------------------------------------

// AC-FC1: calc を付けた作成は 201 で、応答・一覧の calc は既定値を補った形(field・options まで全キー)。
// 任意項目(double・天候・フィールド・壁・急所・個体の任意項目)は往復で落ちない。
func TestCreateFavoriteWithCalcRoundTrips(t *testing.T) {
	t.Run("最小の calc は既定値を補って返る", func(t *testing.T) {
		st := newFakeStore()
		h := NewHandler(st)
		rec := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA),
			favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), calcInput()))
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
		}
		got := decodeFavorite(t, rec, http.StatusCreated)
		if got.Calc == nil {
			t.Fatalf("応答に calc が無い: %s", rec.Body.String())
		}
		calcJSON, err := json.Marshal(rawKeys(t, rec.Body.Bytes())["calc"])
		if err != nil {
			t.Fatal(err)
		}
		if !jsonSemanticallyEqual(t, calcJSON, []byte(wantCalcCanonical)) {
			t.Errorf("応答の calc =\n%s\nwant(既定値を補った形)\n%s", calcJSON, wantCalcCanonical)
		}

		list := decodeFavorites(t, serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil))
		if len(list) != 1 || list[0].Calc == nil || list[0].Calc.MoveId != fakeMove {
			t.Errorf("一覧で calc が返らない: %+v", list)
		}
	})

	t.Run("任意項目まで往復する", func(t *testing.T) {
		st := newFakeStore()
		h := NewHandler(st)
		got := decodeFavorite(t, serve(t, h, http.MethodPost, pathFavorites, headers(deviceA),
			favoriteWithCalcBody(t, "雨ダブル", fullIndividual(speciesGuard), fullCalcInput())), http.StatusCreated)
		c := got.Calc
		if c == nil {
			t.Fatal("calc が無い")
		}
		if c.Format != "double" || c.MoveId != fakeMove {
			t.Errorf("format / moveId = %q / %q, want double / %s", c.Format, c.MoveId, fakeMove)
		}
		if c.Field == nil || c.Field.Weather == nil || *c.Field.Weather != "rain" || c.Field.Terrain == nil || *c.Field.Terrain != "electric" {
			t.Fatalf("field が往復しない: %+v", c.Field)
		}
		as, ds := c.Field.AttackerScreens, c.Field.DefenderScreens
		if as == nil || ds == nil || as.Reflect == nil || !*as.Reflect || as.LightScreen == nil || *as.LightScreen ||
			ds.LightScreen == nil || !*ds.LightScreen || ds.AuroraVeil == nil || !*ds.AuroraVeil || ds.Reflect == nil || *ds.Reflect {
			t.Errorf("壁が往復しない(送らなかった値は false で補う): attacker=%+v defender=%+v", as, ds)
		}
		if c.Options == nil || c.Options.Critical == nil || !*c.Options.Critical {
			t.Errorf("options.critical が往復しない: %+v", c.Options)
		}
		a, d := c.Attacker, c.Defender
		if a.SpeciesKey != speciesGuard || d.SpeciesKey != speciesLeaf {
			t.Errorf("attacker / defender = %s / %s", a.SpeciesKey, d.SpeciesKey)
		}
		if a.AbilityId == nil || *a.AbilityId != "fake-ability" || a.ItemId == nil || *a.ItemId != "fake-item" ||
			a.TeraType == nil || *a.TeraType != "steel" || a.Status == nil || *a.Status != "burn" ||
			a.Ranks == nil || a.Ranks.Atk == nil || *a.Ranks.Atk != 1 || a.Ranks.Spe == nil || *a.Ranks.Spe != -1 {
			t.Errorf("attacker の任意項目が往復しない: %+v", a)
		}
	})
}

// --- AC-FC2: 後方互換(calc 無し) -------------------------------------------

// AC-FC2: calc を送らない(省略・null)作成は従来と同じ: 応答・一覧に "calc" キーを出さない(null も出さない)、
// snapshot のバイト列は ADR-0227 の正規形のまま(既存行の snapshot_hash と重複判定が変わらない)。
func TestFavoriteWithoutCalcKeepsLegacySnapshot(t *testing.T) {
	legacy := `{"label":null,"individual":` + wantAttackerCanonical + `}`
	for _, tt := range []struct {
		name string
		body []byte
	}{
		{"calc を省略", favoriteBody(t, nil, minimalIndividual(speciesGuard))},
		{"calc が null", favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), nil)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			h := NewHandler(st)
			rec := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), tt.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
			}
			if _, ok := rawKeys(t, rec.Body.Bytes())["calc"]; ok {
				t.Errorf("calc の無いお気に入りの応答に calc キーがある(キーごと省く): %s", rec.Body.String())
			}
			rows := st.favoritesOf(deviceA)
			if len(rows) != 1 || string(rows[0].Snapshot) != legacy {
				t.Errorf("snapshot のバイト列が従来の正規形と違う:\ngot  %s\nwant %s", rows[0].Snapshot, legacy)
			}
			listRec := serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil)
			if strings.Contains(listRec.Body.String(), `"calc"`) {
				t.Errorf("一覧に calc キーがある: %s", listRec.Body.String())
			}
		})
	}

	// 保存済みの旧い行(API 以前・calc 導入前の snapshot)もそのまま読め、calc を持たない。
	t.Run("既存の行を読む", func(t *testing.T) {
		st := newFakeStore()
		st.seed(deviceA, 0, 0, 1)
		rec := serve(t, NewHandler(st), http.MethodGet, pathFavorites, headers(deviceA), nil)
		list := decodeFavorites(t, rec)
		if len(list) != 1 || list[0].Calc != nil {
			t.Errorf("既存の行 = %+v, want calc なしの1件", list)
		}
	})
}

// --- AC-FC3: 正規形と冪等 ------------------------------------------------------

// AC-FC3: calc 付きの snapshot は決まった正規形(キー順・既定値の補完)で、同じ内容なら同じバイト列。
// 既定値を明示した calc と省いた calc は同じお気に入り(2回目は 200・同じ id)。
func TestFavoriteCalcSnapshotIsCanonicalAndIdempotent(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	first := createFavoriteRaw(t, h, deviceA, favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), calcInput()), http.StatusCreated)

	rows := st.favoritesOf(deviceA)
	wantSnap := `{"label":null,"individual":` + wantAttackerCanonical + `,"calc":` + wantCalcCanonical + `}`
	if len(rows) != 1 || string(rows[0].Snapshot) != wantSnap {
		t.Fatalf("snapshot が正規形でない:\ngot  %s\nwant %s", rows[0].Snapshot, wantSnap)
	}

	explicit := map[string]any{
		"options":  map[string]any{"critical": false},
		"moveId":   fakeMove,
		"field":    map[string]any{"weather": "none", "terrain": "none", "attackerScreens": map[string]any{}, "defenderScreens": map[string]any{"reflect": false}},
		"defender": minimalIndividual(speciesLeaf),
		"attacker": map[string]any{"status": "none", "level": 50, "ranks": map[string]any{}, "speciesKey": speciesGuard, "natureId": "fake-nature", "sp": spBlock(32, 0, 2, 0, 0, 32), "itemId": nil},
		"format":   "single",
	}
	again := createFavoriteRaw(t, h, deviceA, favoriteWithCalcBody(t, "", minimalIndividual(speciesGuard), explicit), http.StatusOK)
	if again.Id != first.Id {
		t.Errorf("既定値を明示した同じ calc の id = %s, want %s(同じ内容)", again.Id, first.Id)
	}
	if n := len(st.favoritesOf(deviceA)); n != 1 {
		t.Errorf("行数 = %d, want 1", n)
	}
}

// AC-FC3: calc の有無・中身が違えば別のお気に入り(201)。
func TestFavoriteCalcDifferencesAreDistinct(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)
	createFavoriteRaw(t, h, deviceA, favoriteBody(t, nil, minimalIndividual(speciesGuard)), http.StatusCreated)
	createFavoriteRaw(t, h, deviceA, favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), calcInput()), http.StatusCreated)

	otherMove := calcInput()
	otherMove["moveId"] = "fake-move-2"
	createFavoriteRaw(t, h, deviceA, favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), otherMove), http.StatusCreated)

	crit := calcInput()
	crit["options"] = map[string]any{"critical": true}
	createFavoriteRaw(t, h, deviceA, favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), crit), http.StatusCreated)

	if n := len(st.favoritesOf(deviceA)); n != 4 {
		t.Errorf("行数 = %d, want 4(calc 無し・calc あり・技違い・急所違いは別物)", n)
	}
}

// --- AC-FC4: 検証 ---------------------------------------------------------------

// AC-FC4: calc の中も individual と同じ範囲検証(400 invalid_input。境界ちょうどは通る)、列挙の未知の値・
// format の欠落は 400 invalid_enum(calc-svc と同じ)、契約に無いキーは 400 unknown_field。拒否した要求は行を作らない。
func TestCreateFavoriteValidatesCalc(t *testing.T) {
	mut := func(f func(c map[string]any)) map[string]any {
		c := calcInput()
		f(c)
		return c
	}
	attacker := func(f func(m map[string]any)) map[string]any {
		return mut(func(c map[string]any) { m := minimalIndividual(speciesGuard); f(m); c["attacker"] = m })
	}
	defender := func(f func(m map[string]any)) map[string]any {
		return mut(func(c map[string]any) { m := minimalIndividual(speciesLeaf); f(m); c["defender"] = m })
	}

	tests := []struct {
		name     string
		calc     any
		wantCode string // "" なら 201
	}{
		{"境界ちょうど(SP 32・合計66・ランク±6)", attacker(func(m map[string]any) {
			m["sp"] = spBlock(32, 32, 2, 0, 0, 0)
			m["ranks"] = map[string]any{"atk": 6, "spe": -6}
		}), ""},
		{"attacker の SP 33", attacker(func(m map[string]any) { m["sp"] = spBlock(33, 0, 0, 0, 0, 0) }), "invalid_input"},
		{"defender の SP 合計67", defender(func(m map[string]any) { m["sp"] = spBlock(32, 32, 3, 0, 0, 0) }), "invalid_input"},
		{"defender のランク -7", defender(func(m map[string]any) { m["ranks"] = map[string]any{"def": -7} }), "invalid_input"},
		{"attacker の level 51", attacker(func(m map[string]any) { m["level"] = 51 }), "invalid_input"},
		{"defender の speciesKey の形式違い", defender(func(m map[string]any) { m["speciesKey"] = "garchomp" }), "invalid_input"},
		{"attacker の natureId 空", attacker(func(m map[string]any) { m["natureId"] = "" }), "invalid_input"},
		{"attacker の欠落", mut(func(c map[string]any) { delete(c, "attacker") }), "invalid_input"},
		{"defender の欠落", mut(func(c map[string]any) { delete(c, "defender") }), "invalid_input"},
		{"moveId の欠落", mut(func(c map[string]any) { delete(c, "moveId") }), "invalid_input"},
		{"moveId が空", mut(func(c map[string]any) { c["moveId"] = "" }), "invalid_input"},
		{"format の欠落", mut(func(c map[string]any) { delete(c, "format") }), "invalid_enum"},
		{"format が未知", mut(func(c map[string]any) { c["format"] = "triple" }), "invalid_enum"},
		{"weather が未知", mut(func(c map[string]any) { c["field"] = map[string]any{"weather": "fog"} }), "invalid_enum"},
		{"terrain が未知", mut(func(c map[string]any) { c["field"] = map[string]any{"terrain": "swamp"} }), "invalid_enum"},
		{"defender の status が未知", defender(func(m map[string]any) { m["status"] = "confused" }), "invalid_enum"},
		{"attacker の teraType が未知", attacker(func(m map[string]any) { m["teraType"] = "stellar-x" }), "invalid_enum"},
		{"calc に契約に無いキー", mut(func(c map[string]any) { c["memo"] = "x" }), "unknown_field"},
		{"calc に deviceId", mut(func(c map[string]any) { c["deviceId"] = deviceB }), "unknown_field"},
		{"field に契約に無いキー", mut(func(c map[string]any) { c["field"] = map[string]any{"gravity": true} }), "unknown_field"},
		{"壁に契約に無いキー", mut(func(c map[string]any) {
			c["field"] = map[string]any{"attackerScreens": map[string]any{"safeguard": true}}
		}), "unknown_field"},
		{"options に契約に無いキー", mut(func(c map[string]any) { c["options"] = map[string]any{"spread": true} }), "unknown_field"},
		{"attacker に契約に無いキー", attacker(func(m map[string]any) { m["moveIds"] = []string{"m"} }), "unknown_field"},
		{"calc が配列", []any{1}, "invalid_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathFavorites, headers(deviceA),
				favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), tt.calc))
			if tt.wantCode == "" {
				if rec.Code != http.StatusCreated {
					t.Errorf("status = %d, want 201(境界ちょうどは通る); body=%s", rec.Code, rec.Body.String())
				}
				return
			}
			assertErrorBody(t, rec, http.StatusBadRequest, api.ErrorCode(tt.wantCode))
			if n := len(st.favoritesOf(deviceA)); n != 0 {
				t.Errorf("拒否した要求で行が %d 件できた", n)
			}
		})
	}
}

// AC-FC5: calc の ID(moveId・種族・持ち物など)はマスタと照合しない(record-svc は pokedex-svc に依存しない)。
func TestCreateFavoriteCalcDoesNotConsultMaster(t *testing.T) {
	c := calcInput()
	c["moveId"] = "no-such-move"
	d := minimalIndividual("9999-999")
	d["itemId"] = "no-such-item"
	c["defender"] = d
	got := createFavoriteRaw(t, NewHandler(newFakeStore()), deviceA, favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), c), http.StatusCreated)
	if got.Calc == nil || got.Calc.MoveId != "no-such-move" || got.Calc.Defender.SpeciesKey != "9999-999" {
		t.Errorf("マスタに無い ID の calc が保存されない: %+v", got.Calc)
	}
}

// --- AC-FC6: サイズ上限 ---------------------------------------------------------

// AC-FC6: 正規化後の snapshot が 4096 バイトを超える作成は 400 invalid_input(calc の有無にかかわらず)。
// 現実的に最大の入力(ラベル30文字・両側に任意項目・64文字の ID)は通る。
func TestCreateFavoriteSnapshotSizeLimit(t *testing.T) {
	long := strings.Repeat("m", 4096)

	t.Run("moveId が長すぎる", func(t *testing.T) {
		c := calcInput()
		c["moveId"] = long
		st := newFakeStore()
		rec := serve(t, NewHandler(st), http.MethodPost, pathFavorites, headers(deviceA), favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), c))
		assertErrorBody(t, rec, http.StatusBadRequest, api.ErrorCode("invalid_input"))
		if n := len(st.favoritesOf(deviceA)); n != 0 {
			t.Errorf("行が %d 件できた", n)
		}
	})
	t.Run("calc 無しでも individual の ID が長すぎれば拒む", func(t *testing.T) {
		ind := minimalIndividual(speciesGuard)
		ind["natureId"] = long
		rec := serve(t, NewHandler(newFakeStore()), http.MethodPost, pathFavorites, headers(deviceA), favoriteBody(t, nil, ind))
		assertErrorBody(t, rec, http.StatusBadRequest, api.ErrorCode("invalid_input"))
	})
	t.Run("現実的に最大の入力は通る", func(t *testing.T) {
		id64 := strings.Repeat("x", 64)
		big := func(species string) map[string]any {
			m := fullIndividual(species)
			m["natureId"], m["abilityId"], m["itemId"] = id64, id64, id64
			return m
		}
		c := fullCalcInput()
		c["attacker"], c["defender"], c["moveId"] = big(speciesGuard), big(speciesLeaf), id64
		rec := serve(t, NewHandler(newFakeStore()), http.MethodPost, pathFavorites, headers(deviceA),
			favoriteWithCalcBody(t, strings.Repeat("あ", 30), big(speciesGuard), c))
		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
		}
	})
}

// AC-FC6: 正規化後の snapshot がちょうど 4096 バイトなら 201、4097 バイトなら 400(行を作らない)。
// moveId の長さで調整し、保存される snapshot の長さをテスト内で測って境界であることを確かめる。
func TestCreateFavoriteSnapshotSizeBoundary(t *testing.T) {
	snapshotLen := func(moveLen int) int {
		c := calcInput()
		c["moveId"] = strings.Repeat("m", moveLen)
		var req favoriteRequest
		if err := json.Unmarshal(favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), c), &req); err != nil {
			t.Fatal(err)
		}
		snap, err := normalizeFavorite(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(snap)
		if err != nil {
			t.Fatal(err)
		}
		return len(raw)
	}
	base := snapshotLen(1)
	exact := 4096 - base + 1 // moveId の長さ(ASCII 1文字 = 1バイト)
	for _, tt := range []struct {
		name     string
		moveLen  int
		wantSize int
		want     int
	}{
		{"ちょうど4096バイト", exact, 4096, http.StatusCreated},
		{"4097バイト", exact + 1, 4097, http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := snapshotLen(tt.moveLen); got != tt.wantSize {
				t.Fatalf("測った snapshot = %dバイト, want %d", got, tt.wantSize)
			}
			c := calcInput()
			c["moveId"] = strings.Repeat("m", tt.moveLen)
			st := newFakeStore()
			rec := serve(t, NewHandler(st), http.MethodPost, pathFavorites, headers(deviceA), favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), c))
			if tt.want == http.StatusCreated {
				if rec.Code != http.StatusCreated {
					t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
				}
				return
			}
			assertErrorBody(t, rec, http.StatusBadRequest, api.ErrorCode("invalid_input"))
			if n := len(st.favoritesOf(deviceA)); n != 0 {
				t.Errorf("行が %d 件できた", n)
			}
		})
	}
}

// --- AC-FC7: 読めない行 ---------------------------------------------------------

// AC-FC7: 保存済みの calc が計算入力として読めない行があれば、一覧は 500 internal(黙って calc を落とさない。
// ADR-0227 §2 の「読めない行」と同じ)。
func TestListFavoritesWithCorruptCalcIsInternal(t *testing.T) {
	for _, snap := range []string{
		`{"label":null,"individual":` + wantAttackerCanonical + `,"calc":"oops"}`,
		`{"label":null,"individual":` + wantAttackerCanonical + `,"calc":{"format":"single","attacker":{"speciesKey":"bad"},"defender":` + wantDefenderCanonical + `,"moveId":"m"}}`,
	} {
		st := newFakeStore()
		st.mu.Lock()
		st.insertFavoriteLocked(deviceA, speciesGuard, []byte(snap), st.now)
		st.mu.Unlock()
		rec := serve(t, NewHandler(st), http.MethodGet, pathFavorites, headers(deviceA), nil)
		assertErrorBody(t, rec, http.StatusInternalServerError, api.ErrorCode("internal"))
	}
}

// --- AC-FC8: 契約 ----------------------------------------------------------------

// AC-FC8: calc 付きの作成(201)・再ピン留め(200)・一覧(200)の要求と応答が契約どおり。
func TestFavoriteWithCalcMatchesContract(t *testing.T) {
	hdr := headers(deviceA)
	hdr.Set("Content-Type", "application/json")
	for _, tt := range []struct {
		name string
		body []byte
	}{
		{"最小の calc", favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), calcInput())},
		{"任意項目まで", favoriteWithCalcBody(t, "雨ダブル", fullIndividual(speciesGuard), fullCalcInput())},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			h := NewHandler(st)
			rec := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), tt.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
			}
			assertMatchesContract(t, http.MethodPost, pathFavorites, hdr, tt.body, rec, true)

			again := serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), tt.body)
			if again.Code != http.StatusOK {
				t.Fatalf("2回目 status = %d, want 200; body=%s", again.Code, again.Body.String())
			}
			assertMatchesContract(t, http.MethodPost, pathFavorites, hdr, tt.body, again, true)

			list := serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil)
			assertMatchesContract(t, http.MethodGet, pathFavorites, headers(deviceA), nil, list, true)
		})
	}
}

// AC-FC8(契約そのもの): FavoriteInput.calc・Favorite.calc は CalcRequest への $ref だけ(包む型を挟まない)で省略可。
// individual は両方で必須のまま(旧クライアントの互換。ADR-0228 §1)。
func TestContractFavoriteCalcIsOptionalCalcRequest(t *testing.T) {
	doc, _ := loadContract(t)
	for _, name := range []string{"FavoriteInput", "Favorite"} {
		s := doc.Components.Schemas[name].Value
		ref := s.Properties["calc"]
		if ref == nil {
			t.Errorf("%s に calc が無い", name)
			continue
		}
		if ref.Ref != "#/components/schemas/CalcRequest" {
			t.Errorf("%s.calc の $ref = %q, want #/components/schemas/CalcRequest(生成型を共有する)", name, ref.Ref)
		}
		required := map[string]bool{}
		for _, r := range s.Required {
			required[r] = true
		}
		if required["calc"] {
			t.Errorf("%s.calc が必須になっている(旧クライアント・旧い行の互換のため省略可)", name)
		}
		if !required["individual"] {
			t.Errorf("%s.individual が必須でない(旧クライアントが individual を読む)", name)
		}
	}
}

// --- AC-FC9: ログ --------------------------------------------------------------

// AC-FC9: calc の中身(技・相手の種族・持ち物)と本文はログに出さない(成功・400・503 のどの経路でも)。
func TestFavoriteCalcLogsDoNotLeakContent(t *testing.T) {
	const (
		forbiddenMove    = "forbidden-move"
		forbiddenSpecies = "9876-002"
		forbiddenItem    = "forbidden-def-item"
	)
	c := calcInput()
	c["moveId"] = forbiddenMove
	d := minimalIndividual(forbiddenSpecies)
	d["itemId"] = forbiddenItem
	c["defender"] = d
	bad := calcInput()
	bad["moveId"] = forbiddenMove
	bad["format"] = "triple"

	st := newFakeStore()
	buf := captureLogs(t)
	h := NewHandler(st)
	serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), c))
	serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), bad))
	serve(t, h, http.MethodGet, pathFavorites, headers(deviceA), nil)
	st.unavailable = true
	serve(t, h, http.MethodPost, pathFavorites, headers(deviceA), favoriteWithCalcBody(t, nil, minimalIndividual(speciesGuard), c))

	logs := buf.String()
	for _, forbidden := range []string{forbiddenMove, forbiddenSpecies, forbiddenItem, `"calc"`, `"moveId"`} {
		if strings.Contains(logs, forbidden) {
			t.Errorf("ログに出してはいけない値 %q が含まれている(ADR-0209 §3):\n%s", forbidden, logs)
		}
	}
}

// --- 補助 ---------------------------------------------------------------------

func createFavoriteRaw(t *testing.T, h http.Handler, deviceID string, b []byte, wantStatus int) api.Favorite {
	t.Helper()
	return decodeFavorite(t, serve(t, h, http.MethodPost, pathFavorites, headers(deviceID), b), wantStatus)
}

func jsonSemanticallyEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatalf("JSON でない: %v; %s", err, a)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatalf("JSON でない: %v; %s", err, b)
	}
	xb, _ := json.Marshal(x)
	yb, _ := json.Marshal(y)
	return string(xb) == string(yb)
}
