package master_test

// 持ち物・特性の「未対応」の印(UnsupportedAttacker / UnsupportedDefender。issue #270 案 B / ADR-0123)。
// 効果スキーマで表せない持ち物・特性を、効果定義の JSON の中で「その側で持つとダメージが変わるのに
// 計算に入れていない」と示す。ほかのフラグと同じく true だけを認める。

import (
	"errors"
	"reflect"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
)

func TestDecodeUnsupportedEffectMarks(t *testing.T) {
	c := testChart(t)
	cases := []struct {
		raw      string
		wantItem engine.ItemEffect
		wantAb   engine.AbilityEffect
	}{
		{`{"UnsupportedAttacker":true}`, engine.ItemEffect{UnsupportedAttacker: true}, engine.AbilityEffect{UnsupportedAttacker: true}},
		{`{"UnsupportedDefender":true}`, engine.ItemEffect{UnsupportedDefender: true}, engine.AbilityEffect{UnsupportedDefender: true}},
		{`{"UnsupportedAttacker":true,"UnsupportedDefender":true}`,
			engine.ItemEffect{UnsupportedAttacker: true, UnsupportedDefender: true},
			engine.AbilityEffect{UnsupportedAttacker: true, UnsupportedDefender: true}},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			item, err := master.DecodeItemEffect([]byte(tc.raw), c)
			if err != nil || !reflect.DeepEqual(*item, tc.wantItem) {
				t.Errorf("item: %+v, %v; want %+v", item, err, tc.wantItem)
			}
			ab, err := master.DecodeAbilityEffect([]byte(tc.raw), c)
			if err != nil || !reflect.DeepEqual(*ab, tc.wantAb) {
				t.Errorf("ability: %+v, %v; want %+v", ab, err, tc.wantAb)
			}
			// 正準形は UnsupportedAttacker → UnsupportedDefender の順(struct 定義順)。
			enc, err := master.EncodeItemEffect(tc.wantItem)
			if err != nil || string(enc) != tc.raw {
				t.Errorf("EncodeItemEffect = %s, %v; want %s", enc, err, tc.raw)
			}
			enc, err = master.EncodeAbilityEffect(tc.wantAb)
			if err != nil || string(enc) != tc.raw {
				t.Errorf("EncodeAbilityEffect = %s, %v; want %s", enc, err, tc.raw)
			}
		})
	}
}

func TestDecodeUnsupportedEffectMarksRejectsNonTrue(t *testing.T) {
	c := testChart(t)
	for _, raw := range []string{
		`{"UnsupportedAttacker":false}`,
		`{"UnsupportedDefender":"true"}`,
		`{"UnsupportedDefender":1}`,
		`{"unsupportedAttacker":true}`,
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := master.DecodeItemEffect([]byte(raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("item: err = %v, want ErrInvalidEffect", err)
			}
			if _, err := master.DecodeAbilityEffect([]byte(raw), c); !errors.Is(err, master.ErrInvalidEffect) {
				t.Errorf("ability: err = %v, want ErrInvalidEffect", err)
			}
		})
	}
}
