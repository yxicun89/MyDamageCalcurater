package api_test

// 契約に認証の語彙が無いことの検査(ADR-0210 §4.2 の G5。issue #148)。
//
// ユーザー決定(DECISIONS.md 2026-09-23)は「私設サービスを維持する・端末 ID は引き続き認証ではない」。
// 認証を足すのは、その決定を明示的に覆すとき(ADR-0209 §1 の追記・ADR-0210 §6)だけで、
// そのときは認証主体とデータ所有境界を決める別 ADR が先に要る。
//
// このテストは「認証が無言で契約に入る」ことを防ぐ門番で、認証を入れる変更はここで必ず赤くなる。
// 赤くなったら ADR に戻ること(テストを緩めて通さない。CLAUDE.md 絶対ルール6)。
//
// 仕様は生成物に埋め込まれたもの(api.GetSwagger。make gen で openapi.yaml から作られる)を読む。

import (
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
)

// authErrorCodes は「認証・認可がある」ことを意味する ErrorCode の語。1つも無いこと。
var authErrorCodes = []string{
	"unauthorized", "forbidden", "invalid_token", "expired_token", "invalid_credentials", "authentication_required",
}

func TestContractHasNoAuthentication(t *testing.T) {
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("契約(api/openapi.yaml)を読めない: %v", err)
	}

	t.Run("securitySchemes が無い", func(t *testing.T) {
		if doc.Components != nil && len(doc.Components.SecuritySchemes) != 0 {
			var names []string
			for name := range doc.Components.SecuritySchemes {
				names = append(names, name)
			}
			t.Errorf("components/securitySchemes がある: %v(認証は入れない。ADR-0210 §4)", names)
		}
		if len(doc.Security) != 0 {
			t.Errorf("トップレベルの security がある: %v(認証は入れない。ADR-0210 §4)", doc.Security)
		}
	})

	t.Run("操作に security も 401 / 403 も無い", func(t *testing.T) {
		if doc.Paths == nil {
			t.Fatal("paths が無い(契約が空。検査が空振りしている)")
		}
		seen := 0
		for path, item := range doc.Paths.Map() {
			for method, op := range item.Operations() {
				seen++
				if op.Security != nil && len(*op.Security) != 0 {
					t.Errorf("%s %s に security がある(認証は入れない。ADR-0210 §4)", method, path)
				}
				if op.Responses == nil {
					continue
				}
				for _, status := range []string{"401", "403"} {
					if op.Responses.Value(status) != nil {
						t.Errorf("%s %s に %s の応答がある(端末 ID は資格情報ではないので認証の語彙を使わない。"+
							"形式の不正は 400 invalid_header。ADR-0202 §4・ADR-0210 §4)", method, path, status)
					}
				}
			}
		}
		if seen == 0 {
			t.Fatal("操作が1つも無い(検査が空振りしている)")
		}
	})

	t.Run("Authorization / Cookie のパラメータが無い", func(t *testing.T) {
		if doc.Components == nil {
			t.Fatal("components が無い(検査が空振りしている)")
		}
		for name, ref := range doc.Components.Parameters {
			if ref == nil || ref.Value == nil {
				continue
			}
			header := strings.ToLower(ref.Value.Name)
			if ref.Value.In == "header" && (header == "authorization" || header == "cookie") {
				t.Errorf("components/parameters/%s が %s ヘッダを定義している(認証は入れない。ADR-0210 §4)",
					name, ref.Value.Name)
			}
		}
	})

	t.Run("ErrorCode に認証系の code が無い", func(t *testing.T) {
		if doc.Components == nil {
			t.Fatal("components が無い(検査が空振りしている)")
		}
		ref := doc.Components.Schemas["ErrorCode"]
		if ref == nil || ref.Value == nil {
			t.Fatal("components/schemas/ErrorCode が契約に無い")
		}
		if len(ref.Value.Enum) == 0 {
			t.Fatal("ErrorCode の enum が空(検査が空振りしている)")
		}
		for _, v := range ref.Value.Enum {
			code, ok := v.(string)
			if !ok {
				t.Errorf("ErrorCode の enum に文字列でない値がある: %v", v)
				continue
			}
			for _, bad := range authErrorCodes {
				if code == bad {
					t.Errorf("ErrorCode に %q がある(認証・認可の概念を契約に入れない。ADR-0210 §4)", code)
				}
			}
		}
	})
}
