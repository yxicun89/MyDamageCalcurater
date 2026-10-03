package official_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"example.com/pokecalc/apps/wishlist/api/internal/item"
	"example.com/pokecalc/apps/wishlist/api/internal/official"
)

// フェーズ4-3 判定の純関数(docs/phase4-spec.md AC-O9〜O12)。

// AC-O9: Judge は種類がちょうど 1 つのときだけその状態。0 なら unknown、2 つ以上なら ambiguous。
// 長い語を先に消す(予約受付終了 → ended。その中の予約受付・受付終了は数えない)。Neutral の語は数えない。
func TestJudge(t *testing.T) {
	cases := []struct {
		name     string
		text     string
		state    item.OfficialState
		evidence []string
	}{
		// 各状態の語
		{"販売中", "この商品は販売中です", item.OfficialAvailable, []string{"販売中"}},
		{"在庫あり", "在庫あり 送料無料", item.OfficialAvailable, []string{"在庫あり"}},
		{"カートに入れる", "数量 1 カートに入れる", item.OfficialAvailable, []string{"カートに入れる"}},
		{"購入手続きへ", "購入手続きへ進む", item.OfficialAvailable, []string{"購入手続きへ"}},
		{"予約受付中", "ただいま予約受付中", item.OfficialPreorder, []string{"予約受付中"}},
		{"予約する", "予約する", item.OfficialPreorder, []string{"予約する"}},
		{"予約受付", "予約受付 2026年10月10日まで", item.OfficialPreorder, []string{"予約受付"}},
		{"在庫切れ", "在庫切れ", item.OfficialSoldOut, []string{"在庫切れ"}},
		{"売り切れ", "売り切れました", item.OfficialSoldOut, []string{"売り切れ"}},
		{"SOLD OUT", "SOLD OUT", item.OfficialSoldOut, []string{"SOLD OUT"}},
		{"在庫なし", "在庫なし", item.OfficialSoldOut, []string{"在庫なし"}},
		{"販売終了", "販売終了しました", item.OfficialEnded, []string{"販売終了"}},
		{"受付終了", "受付終了", item.OfficialEnded, []string{"受付終了"}},
		{"販売を終了", "本商品は販売を終了いたしました", item.OfficialEnded, []string{"販売を終了"}},

		// 同じ種類の語が複数 → その状態。根拠は現れた順・重複なし・最大 3
		{"同じ種類の複数", "在庫あり カートに入れる 在庫あり", item.OfficialAvailable, []string{"在庫あり", "カートに入れる"}},
		{"根拠は最大 3", "購入手続きへ 販売中 在庫あり カートに入れる", item.OfficialAvailable, []string{"購入手続きへ", "販売中", "在庫あり"}},

		// 長い語を先に消す
		{"予約受付終了 は ended だけ", "予約受付終了", item.OfficialEnded, []string{"予約受付終了"}},
		{"予約受付終了 と 予約受付 が別にある", "予約受付終了 予約受付", item.OfficialAmbiguous, []string{"予約受付終了", "予約受付"}},
		{"予約受付中 は preorder(予約受付を二重に数えない)", "予約受付中", item.OfficialPreorder, []string{"予約受付中"}},
		{"予約受付終了 と 受付終了", "予約受付終了 受付終了", item.OfficialEnded, []string{"予約受付終了", "受付終了"}},

		// Neutral は数えない
		{"販売中止 は available にしない", "販売中止のお知らせ", item.OfficialUnknown, []string{}},
		{"在庫ありません", "在庫ありません", item.OfficialUnknown, []string{}},
		{"予約受付前", "予約受付前です", item.OfficialUnknown, []string{}},
		{"予約受付開始前", "予約受付開始前", item.OfficialUnknown, []string{}},
		{"Neutral と別の語", "販売中止 在庫切れ", item.OfficialSoldOut, []string{"在庫切れ"}},

		// 種類が 2 つ以上 → ambiguous
		{"available と soldout", "カートに入れる 在庫切れ", item.OfficialAmbiguous, []string{"カートに入れる", "在庫切れ"}},
		{"4 種類(根拠は 3 つまで)", "販売中 予約する 売り切れ 販売終了", item.OfficialAmbiguous, []string{"販売中", "予約する", "売り切れ"}},

		// 0 → unknown
		{"語が無い", "仮面ライダーグリス 2026年発売", item.OfficialUnknown, []string{}},
		{"空", "", item.OfficialUnknown, []string{}},

		// 表記の揺れ(NFKC・大文字小文字・空白)
		{"小文字の sold out", "sold out", item.OfficialSoldOut, []string{"SOLD OUT"}},
		{"全角の ＳＯＬＤ ＯＵＴ", "ＳＯＬＤ　ＯＵＴ", item.OfficialSoldOut, []string{"SOLD OUT"}},
		{"空白が続く SOLD  OUT", "SOLD \n  OUT", item.OfficialSoldOut, []string{"SOLD OUT"}},
		{"SOLDOUT(空白なし)は一致しない", "SOLDOUT", item.OfficialUnknown, []string{}},
		{"半角カナは NFKC で一致", "ｶｰﾄに入れる", item.OfficialAvailable, []string{"カートに入れる"}},
		{"語の途中の空白は一致しない", "在庫 あり", item.OfficialUnknown, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := official.Judge(c.text)
			if got.State != c.state || fmt.Sprintf("%q", got.Evidence) != fmt.Sprintf("%q", c.evidence) {
				t.Errorf("Judge(%q) = %s %q, want %s %q", c.text, got.State, got.Evidence, c.state, c.evidence)
			}
			if got.Evidence == nil {
				t.Error("Evidence は空でも nil にしない")
			}
			if len(got.Evidence) > item.MaxOfficialEvidence {
				t.Errorf("根拠が %d 個(最大 %d)", len(got.Evidence), item.MaxOfficialEvidence)
			}
		})
	}
}

// AC-O10: 語の表は spec と同じ(変えるなら spec も変える)。すべての語は MaxOfficialEvidenceLen 以下で、種類をまたいで重複しない。
func TestTerms(t *testing.T) {
	want := map[item.OfficialState]string{
		item.OfficialAvailable: `["販売中" "在庫あり" "カートに入れる" "購入手続きへ"]`,
		item.OfficialPreorder:  `["予約受付中" "予約する" "予約受付"]`,
		item.OfficialSoldOut:   `["在庫切れ" "売り切れ" "SOLD OUT" "在庫なし"]`,
		item.OfficialEnded:     `["販売終了" "受付終了" "予約受付終了" "販売を終了"]`,
	}
	if len(official.Terms) != len(want) {
		t.Fatalf("Terms の種類 = %d", len(official.Terms))
	}
	seen := map[string]bool{}
	for _, ts := range official.Terms {
		if got := fmt.Sprintf("%q", ts.Words); got != want[ts.State] {
			t.Errorf("%s = %s, want %s", ts.State, got, want[ts.State])
		}
		for _, w := range ts.Words {
			if seen[w] || len([]rune(w)) > item.MaxOfficialEvidenceLen {
				t.Errorf("語 %q が重複・長すぎる", w)
			}
			seen[w] = true
		}
	}
	if got := fmt.Sprintf("%q", official.Neutral); got != `["販売中止" "在庫ありません" "予約受付前" "予約受付開始前"]` {
		t.Errorf("Neutral = %s", got)
	}
}

// AC-O11: ExtractText は body のテキスト(script・style・noscript・template を除く)を NFKC にし、空白を詰める。要素の境目は空白。
func TestExtractText(t *testing.T) {
	cases := []struct {
		name, html, want string
	}{
		{"body だけ", `<html><head><title>販売中</title></head><body><p>在庫切れ</p></body></html>`, "在庫切れ"},
		{"script・style・noscript・template を除く", `<body><script>var s="販売中"</script><style>.a:after{content:"在庫あり"}</style><noscript>予約する</noscript><template>販売終了</template><p>SOLD OUT</p></body>`, "SOLD OUT"},
		{"要素の境目は空白", `<body><span>在庫</span><span>あり</span></body>`, "在庫 あり"},
		{"空白を詰める・前後を除く", "<body>\n  予約　受付中 \n\n <b>です</b>\t</body>", "予約 受付中 です"},
		{"NFKC", `<body>ｶｰﾄに入れる ＳＯＬＤ</body>`, "カートに入れる SOLD"},
		{"文字参照", `<body>在庫&nbsp;切れ &amp; 販売中</body>`, "在庫 切れ & 販売中"},
		{"body が無い", `<p>販売終了</p>`, "販売終了"},
		{"空", ``, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := official.ExtractText([]byte(c.html)); got != c.want {
				t.Errorf("ExtractText = %q, want %q", got, c.want)
			}
		})
	}
}

// AC-O12: fixture(架空の公式ページ。internal/official/testdata/)の判定。サイト固有の構造は使わず語だけで決まること。
func TestJudge_Fixtures(t *testing.T) {
	cases := []struct {
		file     string
		state    item.OfficialState
		evidence []string
	}{
		{"preorder.html", item.OfficialPreorder, []string{"予約受付中", "予約する"}},
		{"ended.html", item.OfficialEnded, []string{"予約受付終了"}},
		{"available.html", item.OfficialAvailable, []string{"在庫あり", "カートに入れる"}},
		{"soldout_script_cart.html", item.OfficialSoldOut, []string{"SOLD OUT"}},
		{"ambiguous.html", item.OfficialAmbiguous, []string{"カートに入れる", "在庫切れ"}},
		{"unknown.html", item.OfficialUnknown, []string{}},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", c.file))
			if err != nil {
				t.Fatal(err)
			}
			got := official.Judge(official.ExtractText(b))
			if got.State != c.state || fmt.Sprintf("%q", got.Evidence) != fmt.Sprintf("%q", c.evidence) {
				t.Errorf("%s = %s %q, want %s %q", c.file, got.State, got.Evidence, c.state, c.evidence)
			}
		})
	}
}
