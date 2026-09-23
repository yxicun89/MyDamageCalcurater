package api_test

// 端末 ID / セッション ID の「意味」が契約から消えないための検査(ADR-0209 §2・§6。issue #103)。
//
// ADR-0209 の決定は「端末 ID はデータの分割キーであって認証ではない」「セッション ID は分割キーにしない」で、
// 保存データ(record / team)の分離・削除・保持期間の設計はすべてこの 2 つの前提に乗っている。
// 前提が契約から落ちると、後から読む人が端末 ID を所有権の証明として扱いうるため、
// `api/openapi.yaml` の description に書いてあることをここで固定する(文言そのものではなく、決定を表す語を見る)。
//
// 仕様は生成物に埋め込まれたもの(api.GetSwagger。make gen で openapi.yaml から作られる)を読む。

import (
	"strings"
	"testing"

	"example.com/pokecalc/services/internal/api"
)

func TestClientIDParameterSemantics(t *testing.T) {
	doc, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("契約(api/openapi.yaml)を読めない: %v", err)
	}

	tests := []struct {
		name string
		// parameter は components/parameters の名前。
		parameter  string
		headerName string
		// wantPhrases は description に必ず現れる語(ADR-0209 の決定を表すもの)。
		wantPhrases []string
	}{
		{
			name:       "端末 ID は分割キーであって認証ではない",
			parameter:  "DeviceId",
			headerName: "X-Device-Id",
			// 「分割キー」= 保存データをこの値で分ける / 「認証ではない」= 所有権の証明にしない(ADR-0209 §2)。
			wantPhrases: []string{"分割キー", "認証ではない", "ADR-0209"},
		},
		{
			name:       "セッション ID は分割キーにしない",
			parameter:  "SessionId",
			headerName: "X-Session-Id",
			// 保存データの分離は端末 ID だけで行う(ADR-0209 §2)。
			wantPhrases: []string{"分割キーにはしない", "ADR-0209"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := doc.Components.Parameters[tt.parameter]
			if ref == nil || ref.Value == nil {
				t.Fatalf("components/parameters/%s が契約に無い", tt.parameter)
			}
			param := ref.Value
			if param.Name != tt.headerName {
				t.Errorf("ヘッダ名が %q(期待 %q)", param.Name, tt.headerName)
			}
			if param.In != "header" {
				t.Errorf("in が %q(期待 header)", param.In)
			}
			if !param.Required {
				t.Errorf("required が false(全 /api/* で必須。ADR-0202 §4)")
			}
			for _, phrase := range tt.wantPhrases {
				if !strings.Contains(param.Description, phrase) {
					t.Errorf("description に %q が無い(ADR-0209 の決定が契約から消えている)", phrase)
				}
			}
		})
	}
}
