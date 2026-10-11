package httpapi

// issue #315(メガ部分。ユーザー決定 2026-10-03。ADR-0200 §4 追記): メガシンカ後の種族には
// requiredItemId の持ち物(または持ち物なし=メガストーン扱い)だけを受け付け、別の持ち物は
// 400 invalid_input で拒否する。検証は decode・sp 検査・ID 解決の後。
// サーバーが持ち物候補を展開する経路は無い(省略時は「持ち物なし」の1通り)ので、
// 明示入力(calc の attacker/defender、bulk の attacker・itemVariants、reverse の known・itemCandidates)を検証する。
// 通常種族の応答は変わらない(既存テストがバイト同一を見ている)。

import (
	"net/http"
	"strings"
	"testing"

	"example.com/pokecalc/engine"
)

const (
	speciesMega   = "9002-001" // テストガードメガ(9002-000 のメガ。requiredItemId = itemMegaStone)
	itemMegaStone = "test-guardite"
)

// newMegaStore は fake にメガ種族とメガストーンを足したもの。
func newMegaStore(t testing.TB) *fakeStore {
	t.Helper()
	f := newFakeStore(t)
	base := f.species[speciesDefender]
	base.Key, base.Form, base.NameJa = speciesMega, 1, "テストガードメガ"
	f.species[speciesMega] = base
	f.items[itemMegaStone] = engine.Item{ID: itemMegaStone, NameJa: "テストガードナイト"}
	f.megaItems = map[string]string{speciesMega: itemMegaStone}
	return f
}

func megaIndiv(itemID string) indiv {
	return indiv{speciesKey: speciesMega, natureID: natureNeutral, itemID: itemID, sp: engine.Stats{HP: 32, Def: 32}}
}

func TestCalcMegaSpeciesItemRule(t *testing.T) {
	h := NewHandler(newMegaStore(t), nil)
	tests := []struct {
		name   string
		side   string
		item   string
		status int
	}{
		{"攻撃側メガ+別の持ち物 → 400", "attacker", itemOrb, http.StatusBadRequest},
		{"攻撃側メガ+requiredItemId → 200", "attacker", itemMegaStone, http.StatusOK},
		{"攻撃側メガ+持ち物なし → 200", "attacker", "", http.StatusOK},
		{"防御側メガ+別の持ち物 → 400", "defender", itemOrb, http.StatusBadRequest},
		{"防御側メガ+requiredItemId → 200", "defender", itemMegaStone, http.StatusOK},
		{"防御側メガ+持ち物なし → 200", "defender", "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := calcWith(t, func(b map[string]any) { b[tt.side] = megaIndiv(tt.item).http() })
			rec := post(t, h, "/api/calc", body, false)
			if tt.status == http.StatusOK {
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
				}
				return
			}
			assertError(t, rec, http.StatusBadRequest, "invalid_input")
			msg := decodeErrorBody(t, rec).Message
			if !strings.Contains(msg, speciesMega) || !strings.Contains(msg, tt.item) {
				t.Errorf("message に種族と持ち物を含む: %q", msg)
			}
		})
	}
}

// 通常種族は従来どおり(どの持ち物でも受け付ける)。
func TestCalcNonMegaSpeciesAcceptsAnyItem(t *testing.T) {
	h := NewHandler(newMegaStore(t), nil)
	for _, item := range []string{"", itemOrb, itemMegaStone} {
		body := calcWith(t, func(b map[string]any) {
			attackerOf(b)["itemId"] = item
			if item == "" {
				delete(attackerOf(b), "itemId")
			}
		})
		if rec := post(t, h, "/api/calc", body, false); rec.Code != http.StatusOK {
			t.Errorf("item=%q: status = %d, want 200; body=%s", item, rec.Code, rec.Body.String())
		}
	}
}

// 検証の順序: 未知の持ち物は ID 解決で unknown_item、SP 超過は invalid_input(メガ規則より先)。
func TestCalcMegaRuleRunsAfterIDResolution(t *testing.T) {
	h := NewHandler(newMegaStore(t), nil)
	body := calcWith(t, func(b map[string]any) { b["attacker"] = megaIndiv("test-nothing").http() })
	assertError(t, post(t, h, "/api/calc", body, false), http.StatusBadRequest, "unknown_item")
}

func TestCalcBulkMegaAttackerItemRule(t *testing.T) {
	h := NewHandler(newMegaStore(t), nil)
	for _, tt := range []struct {
		item   string
		status int
	}{{itemOrb, 400}, {itemMegaStone, 200}, {"", 200}} {
		body := bulkBody(movePhysical, nil, nil)
		body["attacker"] = megaIndiv(tt.item).http()
		rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), false)
		if tt.status == 400 {
			assertError(t, rec, http.StatusBadRequest, "invalid_input")
		} else if rec.Code != http.StatusOK {
			t.Errorf("item=%q: status = %d; body=%s", tt.item, rec.Code, rec.Body.String())
		}
	}
}

// bulk の防御側がメガ種族: itemVariants に別の持ち物を含めると 400、省略(持ち物なし1通り)・メガストーン・null は 200。
func TestCalcBulkMegaDefenderItemVariants(t *testing.T) {
	h := NewHandler(newMegaStore(t), nil)
	tests := []struct {
		name     string
		variants any
		status   int
	}{
		{"省略", nil, 200},
		{"null+メガストーン", []any{nil, itemMegaStone}, 200},
		{"別の持ち物を含む", []any{nil, itemOrb}, 400},
		{"別の持ち物だけ", []any{itemOrb}, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bulkBody(movePhysical, nil, tt.variants)
			body["defenderSpeciesKey"] = speciesMega
			rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), false)
			if tt.status == 400 {
				assertError(t, rec, http.StatusBadRequest, "invalid_input")
			} else if rec.Code != http.StatusOK {
				t.Errorf("status = %d; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

// 通常の防御側ではどの持ち物候補も従来どおり 200。
func TestCalcBulkNonMegaDefenderAcceptsAnyVariants(t *testing.T) {
	h := NewHandler(newMegaStore(t), nil)
	body := bulkBody(movePhysical, nil, []any{nil, itemOrb, itemMegaStone})
	if rec := post(t, h, "/api/calc/bulk", mustJSON(t, body), false); rec.Code != http.StatusOK {
		t.Errorf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
}

func TestCalcReverseMegaItemRule(t *testing.T) {
	f := newMegaStore(t)
	h := NewHandler(f, nil)
	base := reverseCases(t, f)[0]

	t.Run("known がメガ+別の持ち物 → 400", func(t *testing.T) {
		b := base.httpBody()
		b["side"] = "attacker"
		b["known"] = megaIndiv(itemOrb).http()
		assertError(t, post(t, h, "/api/calc/reverse", mustJSON(t, b), false), http.StatusBadRequest, "invalid_input")
	})
	t.Run("known がメガ+requiredItemId → 200", func(t *testing.T) {
		b := base.httpBody()
		b["known"] = megaIndiv(itemMegaStone).http()
		if rec := post(t, h, "/api/calc/reverse", mustJSON(t, b), false); rec.Code != http.StatusOK {
			t.Errorf("status = %d; body=%s", rec.Code, rec.Body.String())
		}
	})
	tests := []struct {
		name   string
		items  any
		status int
	}{
		{"unknown がメガ+候補に別の持ち物 → 400", []any{nil, itemOrb}, 400},
		{"unknown がメガ+候補が requiredItemId と null → 200", []any{nil, itemMegaStone}, 200},
		{"unknown がメガ+候補省略 → 200", nil, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := base.httpBody()
			b["unknownSpeciesKey"] = speciesMega
			delete(b, "itemCandidates")
			if tt.items != nil {
				b["itemCandidates"] = tt.items
			}
			rec := post(t, h, "/api/calc/reverse", mustJSON(t, b), false)
			if tt.status == 400 {
				assertError(t, rec, http.StatusBadRequest, "invalid_input")
			} else if rec.Code != http.StatusOK {
				t.Errorf("status = %d; body=%s", rec.Code, rec.Body.String())
			}
		})
	}
	t.Run("通常の推定側は任意の候補で 200(従来どおり)", func(t *testing.T) {
		b := base.httpBody()
		b["itemCandidates"] = []any{nil, itemOrb, itemMegaStone}
		if rec := post(t, h, "/api/calc/reverse", mustJSON(t, b), false); rec.Code != http.StatusOK {
			t.Errorf("status = %d; body=%s", rec.Code, rec.Body.String())
		}
	})
}

// 4) bulk・reverse の 400 もラベル(要素番号)と種族キーをメッセージに含む。
func TestMegaRuleMessagesNameLabelAndSpecies(t *testing.T) {
	f := newMegaStore(t)
	h := NewHandler(f, nil)

	bb := bulkBody(movePhysical, nil, []any{nil, itemOrb})
	bb["defenderSpeciesKey"] = speciesMega
	rec := post(t, h, "/api/calc/bulk", mustJSON(t, bb), false)
	assertError(t, rec, http.StatusBadRequest, "invalid_input")
	m := decodeErrorBody(t, rec).Message
	for _, w := range []string{"itemVariants[1]", speciesMega, itemOrb} {
		if !strings.Contains(m, w) {
			t.Errorf("bulk message %q に %q を含む", m, w)
		}
	}

	ba := bulkBody(movePhysical, nil, nil)
	ba["attacker"] = megaIndiv(itemOrb).http()
	rec = post(t, h, "/api/calc/bulk", mustJSON(t, ba), false)
	m = decodeErrorBody(t, rec).Message
	if !strings.Contains(m, "攻撃側") || !strings.Contains(m, speciesMega) {
		t.Errorf("bulk attacker message = %q", m)
	}

	rb := reverseCases(t, f)[0].httpBody()
	rb["unknownSpeciesKey"] = speciesMega
	rb["itemCandidates"] = []any{nil, itemOrb}
	rec = post(t, h, "/api/calc/reverse", mustJSON(t, rb), false)
	assertError(t, rec, http.StatusBadRequest, "invalid_input")
	m = decodeErrorBody(t, rec).Message
	for _, w := range []string{"itemCandidates[1]", speciesMega, itemOrb} {
		if !strings.Contains(m, w) {
			t.Errorf("reverse message %q に %q を含む", m, w)
		}
	}

	rk := reverseCases(t, f)[0].httpBody()
	rk["known"] = megaIndiv(itemOrb).http()
	rec = post(t, h, "/api/calc/reverse", mustJSON(t, rk), false)
	m = decodeErrorBody(t, rec).Message
	if !strings.Contains(m, "既知の側") || !strings.Contains(m, speciesMega) {
		t.Errorf("reverse known message = %q", m)
	}
}

// 2) メガ違反と SP 合計 67 を同時に送ると、先の段(SP 検査)のメッセージになる。
func TestMegaRuleRunsAfterSPCheck(t *testing.T) {
	h := NewHandler(newMegaStore(t), nil)
	in := megaIndiv(itemOrb)
	in.sp = engine.Stats{Atk: 32, Def: 32, Spe: 3}
	body := calcWith(t, func(b map[string]any) { b["attacker"] = in.http() })
	rec := post(t, h, "/api/calc", body, false)
	assertError(t, rec, http.StatusBadRequest, "invalid_input")
	m := decodeErrorBody(t, rec).Message
	if strings.Contains(m, "メガ") {
		t.Errorf("SP 検査のメッセージのはずがメガ規則: %q", m)
	}
}

// 3) requiredItemId が無いメガ(マスタの欠け。("", true))は持ち物を何も持たせられない。メッセージも言い分ける。
func TestMegaWithoutRequiredItemRejectsAnyItem(t *testing.T) {
	f := newMegaStore(t)
	f.megaItems[speciesMega] = ""
	h := NewHandler(f, nil)
	body := calcWith(t, func(b map[string]any) { b["attacker"] = megaIndiv(itemOrb).http() })
	rec := post(t, h, "/api/calc", body, false)
	assertError(t, rec, http.StatusBadRequest, "invalid_input")
	m := decodeErrorBody(t, rec).Message
	if !strings.Contains(m, "持ち物を持たせられない") || strings.Contains(m, "メガストーン") {
		t.Errorf("message = %q", m)
	}
	body = calcWith(t, func(b map[string]any) { b["attacker"] = megaIndiv("").http() })
	if rec := post(t, h, "/api/calc", body, false); rec.Code != http.StatusOK {
		t.Errorf("持ち物なし: status = %d", rec.Code)
	}
}

// 5) reverse の known がメガ+requiredItemId は side=attacker・defender のどちらも 200。
func TestCalcReverseKnownMegaRequiredItemBothSides(t *testing.T) {
	f := newMegaStore(t)
	h := NewHandler(f, nil)
	for _, c := range reverseCases(t, f) {
		if c.side != engine.SideDefender && c.side != engine.SideAttacker {
			continue
		}
		b := c.httpBody()
		b["known"] = megaIndiv(itemMegaStone).http()
		if rec := post(t, h, "/api/calc/reverse", mustJSON(t, b), false); rec.Code != http.StatusOK {
			t.Errorf("%s side=%s: status = %d; body=%s", c.name, c.side, rec.Code, rec.Body.String())
		}
	}
}
