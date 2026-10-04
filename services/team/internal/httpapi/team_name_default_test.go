package httpapi

// 構築名を省略できる(ADR-0229。usability-round2 F-08「構築名はいらない」)の受け入れテスト。AC-N1〜AC-N6。
//
// **test-first(ADR-0003)**: 実装前に書いた。契約(api/openapi.yaml の TeamInput.name)は
// 「入力では省略・null・空・空白だけを許し、サーバーが既定名『名称未設定』を補う。応答の Team.name は
// 常に1〜50文字の文字列」に変わった。ADR-0213 AC-T2 の「名前1〜50文字」は「名前は0〜50文字、0文字は既定名」へ
// 置き換わる(弱めたのでなく必須→省略可の仕様変更。理由は ADR-0229)。
//
// 実装者へ: 既定名は名前付きの定数1つに置く(handler にリテラルを散らさない)。store・DB(teams.name NOT NULL)は
// 変えない(handler が補ってから store に渡す)。構築名はログに出さない(既定名も含め、名前そのものを出さない)。

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"example.com/pokecalc/services/internal/api"
)

// defaultTeamName は ADR-0229 で決めた既定名。契約(TeamInput.name の description)と同じ文字列。
const defaultTeamName = "名称未設定"

// teamBodyRaw は name の有無・null を自由に組み立てる(teamJSON は name を必ず入れるため)。
func teamBodyRaw(t *testing.T, name any, includeName bool, members ...map[string]any) []byte {
	t.Helper()
	if members == nil {
		members = []map[string]any{}
	}
	m := map[string]any{"members": members}
	if includeName {
		m["name"] = name
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// AC-N1: 作成で name を省略・null・空文字・空白だけ(半角・全角)にしても 201 で、応答・保存・取得の name は既定名。
func TestCreateTeamWithoutNameUsesDefaultName(t *testing.T) {
	tests := []struct {
		name        string
		value       any
		includeName bool
	}{
		{"name を省略", nil, false},
		{"name が null", nil, true},
		{"name が空文字", "", true},
		{"name が半角空白だけ", "   ", true},
		{"name が全角空白だけ", "　　", true},
		{"name が改行とタブだけ", "\n\t", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			h := NewHandler(st)
			created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA),
				teamBodyRaw(t, tt.value, tt.includeName, memberJSON())), http.StatusCreated)
			if created.Name != defaultTeamName {
				t.Errorf("応答の name = %q, want 既定名 %q(ADR-0229)", created.Name, defaultTeamName)
			}
			if len(created.Members) != 1 {
				t.Errorf("members = %d 件, want 1(名前の省略でメンバーを落とさない)", len(created.Members))
			}
			got := decodeTeam(t, serve(t, h, http.MethodGet, teamPath(created.Id), headers(deviceA), nil), http.StatusOK)
			if got.Name != defaultTeamName {
				t.Errorf("取得の name = %q, want 既定名 %q(保存されるのも既定名)", got.Name, defaultTeamName)
			}
			list := decodeTeams(t, serve(t, h, http.MethodGet, pathTeams, headers(deviceA), nil))
			if len(list) != 1 || list[0].Name != defaultTeamName {
				t.Errorf("一覧 = %+v, want 既定名の1件", list)
			}
		})
	}
}

// AC-N2: updateTeam(丸ごと置換)で name を省略・空にすると、前の名前を残さず既定名に置き換わる
// (members の省略が「メンバーなし」への置換なのと同じ。ADR-0213 §2 の PUT の意味を保つ)。
func TestUpdateTeamWithoutNameReplacesWithDefaultName(t *testing.T) {
	for _, tt := range []struct {
		name        string
		value       any
		includeName bool
	}{
		{"name を省略", nil, false},
		{"name が空文字", "", true},
		{"name が null", nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			st := newFakeStore()
			h := NewHandler(st)
			created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA),
				body(t, teamJSON("雨パ", memberJSON(), memberJSON()))), http.StatusCreated)

			updated := decodeTeam(t, serve(t, h, http.MethodPut, teamPath(created.Id), headers(deviceA),
				teamBodyRaw(t, tt.value, tt.includeName, memberJSON())), http.StatusOK)
			if updated.Name != defaultTeamName {
				t.Errorf("置換後の name = %q, want 既定名 %q(前の名前「雨パ」を残さない)", updated.Name, defaultTeamName)
			}
			if len(updated.Members) != 1 {
				t.Errorf("members = %d 件, want 1(置換)", len(updated.Members))
			}
			if !updated.CreatedAt.Equal(created.CreatedAt) {
				t.Errorf("createdAt が変わった: %v → %v", created.CreatedAt, updated.CreatedAt)
			}
		})
	}
}

// AC-N3: 名前を送る従来の要求は従来どおり(前後の空白を除いて保存・50文字ちょうどは通り51文字は 400)。
// 既存クライアント(名前の入力欄を持つ Web・iOS)の挙動を変えない。
func TestNamedTeamKeepsExistingBehavior(t *testing.T) {
	st := newFakeStore()
	h := NewHandler(st)

	trimmed := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA),
		body(t, teamJSON("  雨パ　"))), http.StatusCreated)
	if trimmed.Name != "雨パ" {
		t.Errorf("name = %q, want 雨パ(前後の空白を除く)", trimmed.Name)
	}

	fifty := strings.Repeat("あ", 50)
	ok := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON(fifty))), http.StatusCreated)
	if ok.Name != fifty {
		t.Errorf("50文字の name が往復しない: %q", ok.Name)
	}

	before := st.rowsLeft(deviceA)
	rec := serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON(strings.Repeat("あ", 51))))
	assertErrorBody(t, rec, http.StatusBadRequest, api.InvalidInput)
	if st.rowsLeft(deviceA) != before {
		t.Error("51文字の名前で行が増えた")
	}

	// 利用者が既定名と同じ文字列を明示しても、そのまま保存される(区別はしない。ADR-0229 §1)。
	same := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA), body(t, teamJSON(defaultTeamName))), http.StatusCreated)
	if same.Name != defaultTeamName {
		t.Errorf("name = %q, want %q", same.Name, defaultTeamName)
	}
}

// AC-N4: 既定名そのものが契約の Team.name(1〜50文字)を満たす(既定名を変えるときの網)。
func TestDefaultTeamNameFitsContract(t *testing.T) {
	doc, _ := loadContract(t)
	team := doc.Components.Schemas["Team"].Value
	nameSchema := team.Properties["name"].Value
	if err := nameSchema.VisitJSON(defaultTeamName); err != nil {
		t.Errorf("既定名 %q が契約の Team.name を満たさない: %v", defaultTeamName, err)
	}
	if !strings.Contains(doc.Components.Schemas["TeamInput"].Value.Properties["name"].Value.Description, defaultTeamName) {
		t.Errorf("契約の TeamInput.name の説明に既定名 %q が書かれていない(クライアントが既定名を知る唯一の正)", defaultTeamName)
	}
}

// AC-N5(契約): TeamInput.name は省略可(required に無い・nullable)、Team.name は必須の string(minLength 1)のまま。
// 応答の型を変えると Web・iOS の生成型が壊れる(Team.name を読む既存の画面)ので、応答側は変えない。
func TestContractTeamNameIsOptionalOnInputOnly(t *testing.T) {
	doc, _ := loadContract(t)
	in := doc.Components.Schemas["TeamInput"].Value
	for _, r := range in.Required {
		if r == "name" {
			t.Error("TeamInput.required に name がある(ADR-0229 で省略可)")
		}
	}
	inName := in.Properties["name"].Value
	if !inName.Nullable {
		t.Error("TeamInput.name が nullable でない(null も「未設定」として受ける)")
	}
	if inName.MinLength != 0 {
		t.Errorf("TeamInput.name.minLength = %d, want 0(空文字を契約で弾かない)", inName.MinLength)
	}
	if inName.MaxLength == nil || *inName.MaxLength != 50 {
		t.Errorf("TeamInput.name.maxLength = %v, want 50(上限は変えない)", inName.MaxLength)
	}

	out := doc.Components.Schemas["Team"].Value
	if !containsString(out.Required, "name") {
		t.Error("Team.required に name が無い(応答の name は常に返す)")
	}
	outName := out.Properties["name"].Value
	if outName.Type == nil || !outName.Type.Is(openapi3.TypeString) || outName.Nullable || outName.MinLength != 1 {
		t.Errorf("Team.name = type %v nullable %v minLength %d, want string・null 不可・minLength 1", outName.Type, outName.Nullable, outName.MinLength)
	}
}

// AC-N5(契約の往復): 名前なしの作成・置換の要求と応答が契約どおり。
func TestTeamWithoutNameMatchesContract(t *testing.T) {
	noName := teamBodyRaw(t, nil, false, memberJSON())
	nullName := teamBodyRaw(t, nil, true, memberJSON())

	t.Run("作成 201(名前なし)", func(t *testing.T) {
		rec := serve(t, NewHandler(newFakeStore()), http.MethodPost, pathTeams, headers(deviceA), noName)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
		}
		assertMatchesContract(t, http.MethodPost, pathTeams, headers(deviceA), noName, rec, true)
	})
	t.Run("作成 201(名前 null)", func(t *testing.T) {
		rec := serve(t, NewHandler(newFakeStore()), http.MethodPost, pathTeams, headers(deviceA), nullName)
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
		}
		assertMatchesContract(t, http.MethodPost, pathTeams, headers(deviceA), nullName, rec, true)
	})
	t.Run("更新 200(名前なし)", func(t *testing.T) {
		st := newFakeStore()
		st.seed(deviceA, 1, 1)
		path := pathTeams + "/" + fakeTeamID(1)
		rec := serve(t, NewHandler(st), http.MethodPut, path, headers(deviceA), noName)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		assertMatchesContract(t, http.MethodPut, path, headers(deviceA), noName, rec, true)
	})
}

// AC-N6: 名前を省略した要求でも、構築の中身・既定名を含めて構築名はログに出さない(ADR-0209 §3。AC-L1 の延長)。
// 既定名を出さないのは「名前を出さない」規則を名前の出どころで分けないため。
func TestTeamWithoutNameDoesNotLogName(t *testing.T) {
	buf := captureLogs(t)
	st := newFakeStore()
	h := NewHandler(st)
	created := decodeTeam(t, serve(t, h, http.MethodPost, pathTeams, headers(deviceA), teamBodyRaw(t, nil, false, memberJSON())), http.StatusCreated)
	serve(t, h, http.MethodPut, teamPath(created.Id), headers(deviceA), teamBodyRaw(t, "", true))
	st.unavailable = true
	serve(t, h, http.MethodPost, pathTeams, headers(deviceA), teamBodyRaw(t, nil, false))
	if logs := buf.String(); strings.Contains(logs, defaultTeamName) {
		t.Errorf("ログに構築名(既定名)が出ている:\n%s", logs)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
