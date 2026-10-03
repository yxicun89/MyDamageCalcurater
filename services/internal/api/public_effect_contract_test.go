package api_test

// 公開 API の Item / Ability の effect(issue #211・ADR-0218)の契約の形の検査。
//
//   - effect は省略可(required に入れない)。古いクライアント・古いサーバーとの互換のため(ADR-0218 §2)。
//   - effect の形は内部 API の MasterEffect をそのまま参照する(公開用に別の効果スキーマを定義しない)。
//     効果の形の正は共通マスタ(services/internal/master)の1か所で、契約も1か所に保つ(二重定義しない)。
//
// 仕様は生成物に埋め込まれたもの(api.GetSpec)を読む。

import (
	"slices"
	"testing"

	"example.com/pokecalc/services/internal/api"
)

func TestPublicItemAndAbilityEffectContract(t *testing.T) {
	doc, err := api.GetSpec()
	if err != nil {
		t.Fatalf("契約(api/openapi.yaml)を読めない: %v", err)
	}
	const masterEffectRef = "#/components/schemas/MasterEffect"
	for _, name := range []string{"Item", "Ability"} {
		t.Run(name, func(t *testing.T) {
			ref, ok := doc.Components.Schemas[name]
			if !ok || ref.Value == nil {
				t.Fatalf("components/schemas/%s が無い", name)
			}
			s := ref.Value
			if slices.Contains(s.Required, "effect") {
				t.Errorf("%s.effect が required に入っている(省略可にすること。ADR-0218)", name)
			}
			prop, ok := s.Properties["effect"]
			if !ok || prop.Value == nil {
				t.Fatalf("%s.effect が無い", name)
			}
			refs := []string{prop.Ref}
			for _, a := range prop.Value.AllOf {
				refs = append(refs, a.Ref)
			}
			if !slices.Contains(refs, masterEffectRef) {
				t.Errorf("%s.effect が %s を参照していない(refs=%v。公開用に別の効果スキーマを作らない)", name, masterEffectRef, refs)
			}
		})
	}
}
