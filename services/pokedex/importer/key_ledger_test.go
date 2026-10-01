package importer_test

// 種族 key・技/持ち物/特性 ID の「消滅」と「消滅後の再利用」の検出(issue #277・ADR-0131)の、
// DB を使わない部分のテスト。DB を使う2段階のシナリオは key_ledger_mysql_test.go。

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func rid(kind importer.IDKind, id string) importer.RemovedID {
	return importer.RemovedID{Kind: kind, ID: id}
}

func TestRemovedIDString(t *testing.T) {
	tests := []struct {
		in   importer.RemovedID
		want string
	}{
		{rid(importer.IDKindSpecies, "9002-002"), "species:9002-002"},
		{rid(importer.IDKindMove, "teststrike"), "move:teststrike"},
		{rid(importer.IDKindItem, "testorb"), "item:testorb"},
		{rid(importer.IDKindAbility, "testguard"), "ability:testguard"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// -allow-removed の値(<種類>:<ID> をカンマ区切り)を読む。String() と往復できる形にする。
func TestParseAllowRemoved(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []importer.RemovedID
	}{
		{"空なら何も許さない", "", nil},
		{"種族1件", "species:9002-002", []importer.RemovedID{rid(importer.IDKindSpecies, "9002-002")}},
		{
			"複数の種類・前後の空白",
			"species:9002-002, move:teststrike ,item:testorb,ability:testguard",
			[]importer.RemovedID{
				rid(importer.IDKindSpecies, "9002-002"), rid(importer.IDKindMove, "teststrike"),
				rid(importer.IDKindItem, "testorb"), rid(importer.IDKindAbility, "testguard"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := importer.ParseAllowRemoved(tt.in)
			if err != nil {
				t.Fatalf("ParseAllowRemoved(%q): %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseAllowRemoved(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseAllowRemovedRejectsMalformed(t *testing.T) {
	for _, in := range []string{
		"9002-002",                          // 種類が無い(技 ID と種族 key を取り違えないよう、種類は必須)
		"pokemon:9002-002",                  // 未知の種類
		"species:",                          // ID が空
		":9002-002",                         // 種類が空
		"species:9002-002,,move:teststrike", // 空の要素
		"species:9002-002,species:9002-002", // 重複(打ち間違いの兆候)
		"species:9002-002:extra",            // 区切りが多い
	} {
		t.Run(in, func(t *testing.T) {
			if _, err := importer.ParseAllowRemoved(in); !errors.Is(err, importer.ErrInvalidInput) {
				t.Fatalf("ParseAllowRemoved(%q) err = %v, want ErrInvalidInput", in, err)
			}
		})
	}
}

// RemovedIDs は「投入前の DB にあって、新しい出力に無い」ID を、種類・ID の順に並べて返す。
func TestRemovedIDs(t *testing.T) {
	existing := importer.ExistingIDs{
		SpeciesKeys: []string{"9001-000", "9002-002", "9002-000"},
		MoveIDs:     []string{"teststrike", "testsplash"},
		ItemIDs:     []string{"testorb"},
		AbilityIDs:  []string{"testguard", "testhidden"},
	}
	out := importer.Output{
		Species:   []importer.SpeciesRow{{Key: "9001-000"}, {Key: "9002-000"}, {Key: "9003-000"}},
		Moves:     []importer.MoveRow{{ID: "teststrike"}},
		Items:     []importer.NamedRow{{ID: "testorb"}, {ID: "testnew"}},
		Abilities: []importer.NamedRow{{ID: "testguard"}},
	}
	got := importer.RemovedIDs(existing, out)
	want := []importer.RemovedID{
		rid(importer.IDKindAbility, "testhidden"),
		rid(importer.IDKindMove, "testsplash"),
		rid(importer.IDKindSpecies, "9002-002"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RemovedIDs = %+v, want %+v", got, want)
	}

	// 初回(DB が空)・同じ集合・追加だけなら、消えたものは無い。
	if got := importer.RemovedIDs(importer.ExistingIDs{}, out); len(got) != 0 {
		t.Errorf("DB が空なのに消えたものがある: %+v", got)
	}
	same := importer.ExistingIDs{SpeciesKeys: []string{"9001-000"}, MoveIDs: []string{"teststrike"}}
	if got := importer.RemovedIDs(same, out); len(got) != 0 {
		t.Errorf("追加だけなのに消えたものがある: %+v", got)
	}
}

// CheckRemovals: 許していない消滅は ErrKeyRemoved(*RemovedIDsError で一覧が取れる)。
// 許した ID が実際には消えていないなら、打ち間違いとして ErrInvalidInput。
func TestCheckRemovals(t *testing.T) {
	removed := []importer.RemovedID{rid(importer.IDKindMove, "testsplash"), rid(importer.IDKindSpecies, "9002-002")}

	if err := importer.CheckRemovals(nil, nil); err != nil {
		t.Errorf("消滅なし・許可なし: %v", err)
	}
	if err := importer.CheckRemovals(removed, removed); err != nil {
		t.Errorf("すべて許した: %v", err)
	}

	err := importer.CheckRemovals(removed, []importer.RemovedID{rid(importer.IDKindMove, "testsplash")})
	if !errors.Is(err, importer.ErrKeyRemoved) {
		t.Fatalf("一部だけ許した: err = %v, want ErrKeyRemoved", err)
	}
	var re *importer.RemovedIDsError
	if !errors.As(err, &re) {
		t.Fatalf("err = %T, want *RemovedIDsError を包む", err)
	}
	if want := []importer.RemovedID{rid(importer.IDKindSpecies, "9002-002")}; !reflect.DeepEqual(re.IDs, want) {
		t.Errorf("許していない消滅 = %+v, want %+v", re.IDs, want)
	}
	// 人がそのまま -allow-removed に写せるよう、メッセージに <種類>:<ID> を含める。
	if !strings.Contains(err.Error(), "species:9002-002") {
		t.Errorf("エラー文に species:9002-002 が無い: %v", err)
	}

	err = importer.CheckRemovals(removed, append(append([]importer.RemovedID(nil), removed...), rid(importer.IDKindItem, "testtypo")))
	if !errors.Is(err, importer.ErrInvalidInput) {
		t.Fatalf("消えていない ID を許した: err = %v, want ErrInvalidInput", err)
	}
	if !strings.Contains(err.Error(), "item:testtypo") {
		t.Errorf("エラー文に item:testtypo が無い: %v", err)
	}
}

func TestErrKeyRemovedIsDistinct(t *testing.T) {
	if importer.ErrKeyRemoved == nil {
		t.Fatal("ErrKeyRemoved が nil")
	}
	for _, other := range []error{importer.ErrKeyChanged, importer.ErrInvalidInput, importer.ErrInvalidData, importer.ErrBlocked, importer.ErrSchemaNotReady} {
		if errors.Is(importer.ErrKeyRemoved, other) || errors.Is(other, importer.ErrKeyRemoved) {
			t.Errorf("ErrKeyRemoved が %v と区別できない", other)
		}
	}
}

// CheckLedger: 台帳(過去に配った key と showdown_id の対応。消えた種族も残る)と新しい出力が
// 食い違えば ErrKeyChanged。台帳にある組の再登場(復活)と、新しい組の追加は通す。
func TestCheckLedger(t *testing.T) {
	ledger := []importer.LedgerEntry{
		{Key: "9002-000", ShowdownID: "testleaf"},
		{Key: "9002-002", ShowdownID: "testleafrain"}, // 既に DB から消えた種族も台帳には残る
	}
	tests := []struct {
		name    string
		species []importer.SpeciesRow
		wantErr bool
	}{
		{"台帳どおり", []importer.SpeciesRow{{Key: "9002-000", ShowdownID: "testleaf"}}, false},
		{"消えた種族が同じ key で復活", []importer.SpeciesRow{{Key: "9002-002", ShowdownID: "testleafrain"}}, false},
		{"新しい key と showdown_id の追加", []importer.SpeciesRow{{Key: "9003-000", ShowdownID: "testnew"}}, false},
		{"消えた key を別の showdown_id が再利用", []importer.SpeciesRow{{Key: "9002-002", ShowdownID: "testleafnew"}}, true},
		{"消えた showdown_id が別の key で再登場", []importer.SpeciesRow{{Key: "9002-009", ShowdownID: "testleafrain"}}, true},
		{"今ある key の乗っ取り", []importer.SpeciesRow{{Key: "9002-000", ShowdownID: "testleafrain"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := importer.CheckLedger(ledger, importer.Output{Species: tt.species})
			if tt.wantErr {
				if !errors.Is(err, importer.ErrKeyChanged) {
					t.Fatalf("err = %v, want ErrKeyChanged", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}
