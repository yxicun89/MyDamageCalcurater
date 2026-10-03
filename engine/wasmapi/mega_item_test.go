package wasmapi_test

// issue #505(ADR-0321。calc-svc 側は issue #315・ADR-0200 §4 追記): メガシンカ後の種族(isMega)には
// requiredItemId の持ち物か持ち物なしだけを受け付け、別の持ち物は invalid_input で拒否する。
// 対象は calc の attacker・defender、bulk の attacker と itemVariants(防御側がメガ種族のとき)、
// reverse の known と itemCandidates(推定側がメガ種族のとき)。通常種族は何も検査しない。
// 検証は DTO 変換と SP 検査(invalid_input)の後。

import (
	"strings"
	"testing"
)

const (
	testMegaKey   = "mega-snorlax"
	testMegaStone = "snorlaxite"
	testOtherItem = "lifeorb"
)

// megaSpecies は isMega・requiredItemId を持つ種族(requiredItemId の空はメガストーンなし)。
func megaSpecies(base map[string]any, requiredItemID any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	out["key"] = testMegaKey
	out["isMega"] = true
	out["requiredItemId"] = requiredItemID
	return out
}

func itemValue(id string) any {
	if id == "" {
		return nil
	}
	return map[string]any{"id": id, "nameJa": id}
}

func expectMegaItemError(t *testing.T, fn, req string, wantInMessage ...string) {
	t.Helper()
	e := decodeError(t, invoke(t, fn, req))
	if e.Code != "invalid_input" {
		t.Fatalf("code = %q, want invalid_input; message=%q", e.Code, e.Message)
	}
	for _, w := range wantInMessage {
		if !strings.Contains(e.Message, w) {
			t.Errorf("message %q に %q が無い", e.Message, w)
		}
	}
}

func expectOK(t *testing.T, fn, req string) {
	t.Helper()
	decodeSuccess(t, invoke(t, fn, req))
}

func TestMegaItemRuleCalc(t *testing.T) {
	for _, side := range []string{"attacker", "defender"} {
		for _, tt := range []struct {
			name string
			item string
			ok   bool
		}{
			{"別の持ち物", testOtherItem, false},
			{"requiredItemId", testMegaStone, true},
			{"持ち物なし", "", true},
		} {
			t.Run(side+"/"+tt.name, func(t *testing.T) {
				r := baseCalc()
				ind := sub(t, r, side)
				ind["species"] = megaSpecies(ind["species"].(map[string]any), testMegaStone)
				ind["item"] = itemValue(tt.item)
				if tt.ok {
					expectOK(t, "calc", mustJSON(t, r))
					return
				}
				expectMegaItemError(t, "calc", mustJSON(t, r), testMegaKey, tt.item)
			})
		}
	}
}

func TestMegaItemRuleNonMegaAcceptsAnyItem(t *testing.T) {
	r := baseCalc()
	sub(t, r, "attacker")["item"] = itemValue(testOtherItem)
	expectOK(t, "calc", mustJSON(t, r))
	// isMega=false に requiredItemId があっても検査しない。
	sp := sub(t, sub(t, r, "attacker"), "species")
	sp["isMega"] = false
	sp["requiredItemId"] = testMegaStone
	expectOK(t, "calc", mustJSON(t, r))
}

// requiredItemId が無い(null・省略・空)メガ種族は、どの持ち物も持てない。
func TestMegaItemRuleWithoutRequiredItem(t *testing.T) {
	for name, required := range map[string]any{"null": nil, "空文字": ""} {
		t.Run(name, func(t *testing.T) {
			r := baseCalc()
			ind := sub(t, r, "attacker")
			ind["species"] = megaSpecies(ind["species"].(map[string]any), required)
			ind["item"] = itemValue(testOtherItem)
			expectMegaItemError(t, "calc", mustJSON(t, r), testMegaKey, testOtherItem)
			ind["item"] = nil
			expectOK(t, "calc", mustJSON(t, r))
		})
	}
}

func TestMegaItemRuleBulk(t *testing.T) {
	t.Run("attacker", func(t *testing.T) {
		r := baseBulk()
		a := sub(t, r, "attacker")
		a["species"] = megaSpecies(a["species"].(map[string]any), testMegaStone)
		a["item"] = itemValue(testOtherItem)
		expectMegaItemError(t, "calcBulk", mustJSON(t, r), "攻撃側", testMegaKey, testOtherItem)
		a["item"] = itemValue(testMegaStone)
		expectOK(t, "calcBulk", mustJSON(t, r))
	})
	t.Run("itemVariants(防御側がメガ)", func(t *testing.T) {
		r := baseBulk()
		r["defenderSpecies"] = megaSpecies(r["defenderSpecies"].(map[string]any), testMegaStone)
		r["itemVariants"] = []any{nil, itemValue(testMegaStone)}
		expectOK(t, "calcBulk", mustJSON(t, r))
		r["itemVariants"] = []any{nil, itemValue(testOtherItem), itemValue(testMegaStone)}
		expectMegaItemError(t, "calcBulk", mustJSON(t, r), "itemVariants[1]", testMegaKey, testOtherItem)
	})
	t.Run("itemVariants(防御側が通常)", func(t *testing.T) {
		r := baseBulk()
		r["itemVariants"] = []any{nil, itemValue(testOtherItem), itemValue(testMegaStone)}
		expectOK(t, "calcBulk", mustJSON(t, r))
	})
}

func TestMegaItemRuleReverse(t *testing.T) {
	t.Run("known", func(t *testing.T) {
		r := baseReverse()
		k := sub(t, r, "known")
		k["species"] = megaSpecies(k["species"].(map[string]any), testMegaStone)
		k["item"] = itemValue(testOtherItem)
		expectMegaItemError(t, "calcReverse", mustJSON(t, r), "既知の側", testMegaKey, testOtherItem)
		k["item"] = itemValue(testMegaStone)
		expectOK(t, "calcReverse", mustJSON(t, r))
	})
	t.Run("itemCandidates(推定側がメガ)", func(t *testing.T) {
		r := baseReverse()
		r["unknownSpecies"] = megaSpecies(r["unknownSpecies"].(map[string]any), testMegaStone)
		r["itemCandidates"] = []any{nil, itemValue(testMegaStone)}
		expectOK(t, "calcReverse", mustJSON(t, r))
		r["itemCandidates"] = []any{nil, itemValue(testOtherItem)}
		expectMegaItemError(t, "calcReverse", mustJSON(t, r), "itemCandidates[1]", testMegaKey, testOtherItem)
	})
	t.Run("itemCandidates(推定側が通常)", func(t *testing.T) {
		r := baseReverse()
		r["itemCandidates"] = []any{nil, itemValue(testOtherItem), itemValue(testMegaStone)}
		expectOK(t, "calcReverse", mustJSON(t, r))
	})
}

// 検証は SP 検査の後: SP 超過と別の持ち物が重なっても、どちらも invalid_input だが
// メッセージは SP 超過(先)になる。
func TestMegaItemRuleRunsAfterSPCheck(t *testing.T) {
	r := baseCalc()
	a := sub(t, r, "attacker")
	a["species"] = megaSpecies(a["species"].(map[string]any), testMegaStone)
	a["item"] = itemValue(testOtherItem)
	a["sp"] = stats(0, 32, 32, 32, 0, 0)
	e := decodeError(t, invoke(t, "calc", mustJSON(t, r)))
	if e.Code != "invalid_input" || strings.Contains(e.Message, testOtherItem) {
		t.Errorf("SP 超過が先に報告されるはず: %+v", e)
	}
}
