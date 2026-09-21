//go:build allspecies

package engine

// 逆算の Recall 合格基準(P1-12 で再設計。ADR-0010 §R5)。docs/test-strategy.md「逆算(調整推定)のテスト」:
//
//	1. 既知の調整(SP配分・性格・持ち物)でダメージを生成
//	2. 乱数の1段階を選び、ゲーム内表示と同じ丸め(HP%)をかけて観測値にする
//	   → 実機の丸めは未確認なので、切り捨て・四捨五入・切り上げの3通りすべてで作り、それぞれ判定する
//	3. 逆算にかけ、正解が候補に入る割合(Recall)を測る
//	- 合格基準: 1回観測で Recall >= 80%、2回観測で >= 95%(数値は旧 Recall@5 から変えない)
//	- 全ポケモンからランダムに 1,000 ケース(固定シード)
//
// 新しい Recall の「正解に入る」(ADR-0010 §R5):
//   - 真値の (性格クラス, 持ち物) の候補が説明可能(Exact)で、真値の SP がその Ranges に入り、
//   - かつ、その Ranges が総当たりの正解(観測を説明できる SP の集合)と完全に一致する。
//
// 後者が「全範囲 0..32 を返せば必ず当たる」を防ぐ。さらに、被覆(真値を落とさない)と
// 厳密性(全候補が正解と一致)と絞り込み(観測を足して増えない)は決定的な性質なので 100% を要求する。
// 旧 Recall@5 の「上位5件」は、候補が 性格2 × 持ち物 に縮んだため条件として意味を失った
// (持ち物2なら候補4件で自明、実際の持ち物分類なら区別できない候補の決め打ちを強いる。ADR-0010 §R5)。
// 順位は参考値としてログに出す。
//
// **基準は緩めない**(CLAUDE.md 絶対ルール6)。しきい値・シード・ケース数を動かして通してはならない。

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// reverseRecallSeed は固定シード。**テストを通すために変えない**(絶対ルール6)。
const reverseRecallSeed uint64 = 0x50314238 // "P1B8"

// reverseRecallCases は test-strategy が定める 1,000 ケース。減らさない。
const reverseRecallCases = 1000

// TestAllSpeciesReverseRecall は全種族・固定シード 1,000 ケースで Recall を測る。
func TestAllSpeciesReverseRecall(t *testing.T) {
	species := loadAllSpeciesForRecall(t)
	if len(species) < 1000 {
		t.Fatalf("種族数 = %d。全ポケモンの母集団になっていない", len(species))
	}
	t.Logf("種族数 = %d", len(species))

	tests := []struct {
		side ReverseSide
		nObs int
		want int // %
	}{
		{SideDefender, 1, 80},
		{SideDefender, 2, 95},
		{SideAttacker, 1, 80},
		{SideAttacker, 2, 95},
	}
	for _, tt := range tests {
		for _, rule := range allObsRoundings {
			t.Run(fmt.Sprintf("%s/観測%d件/%s", tt.side, tt.nObs, rule), func(t *testing.T) {
				st := reverseRecall(t, tt.side, species, reverseRecallCases, tt.nObs, reverseRecallSeed, rule)
				if st.total != reverseRecallCases {
					t.Fatalf("ケース数 = %d, want %d", st.total, reverseRecallCases)
				}
				t.Logf("side=%s 観測%d件 %s: %s", tt.side, tt.nObs, rule, st)
				if got := 100 * st.hit / st.total; got < tt.want {
					t.Errorf("side=%s 観測%d件 %s の Recall = %d%% (%d/%d), want >= %d%%"+
						"(しきい値・シード・ケース数を変えて通さないこと)", tt.side, tt.nObs, rule, got, st.hit, st.total, tt.want)
				}
				// 被覆・厳密性・絞り込みは決定的な性質。1件でも破れたら実装の誤り。
				if st.covered != st.total {
					t.Errorf("被覆 = %d/%d(真値の SP を範囲から落とした。区間モデルが丸め規則 %s を覆っていない)",
						st.covered, st.total, rule)
				}
				if st.tightViolations != 0 {
					t.Errorf("厳密性違反 = %d 件(範囲が観測を説明できる SP の集合と一致しない)", st.tightViolations)
				}
				if st.narrowViolations != 0 {
					t.Errorf("絞り込み違反 = %d 件(観測を足して説明可能な SP が増えた)", st.narrowViolations)
				}
			})
		}
	}
}

// loadAllSpeciesForRecall は testdata/golden の防御側網羅ベクタから全種族を取り出す。
// 逆算用に別の種族表を作らず、ゴールデンと同じ母集団を使う(ADR-0010 §7 item 1。P1-12 でも変えない)。
// Key 昇順に並べ、固定シードでの抽出を決定的にする。
func loadAllSpeciesForRecall(t *testing.T) []Species {
	t.Helper()
	const dir = "../testdata/golden"
	const name = "defense-species.jsonl.gz"

	metaRaw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatalf("golden fixtures required (cd tools/golden && npm ci && npm run generate): %v", err)
	}
	var meta struct {
		SpeciesCount int `json:"speciesCount"`
		Files        map[string]struct {
			Count  int    `json:"count"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		t.Fatal(err)
	}
	spec, ok := meta.Files[name]
	if !ok {
		t.Fatalf("fixture %s missing from manifest", name)
	}
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if sum := sha256.Sum256(raw); hex.EncodeToString(sum[:]) != spec.SHA256 {
		t.Fatalf("%s checksum differs; regenerate fixtures and manifest together", name)
	}

	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 4096), 1024*1024)
	seen := map[string]bool{}
	var out []Species
	for sc.Scan() {
		var line struct {
			Input struct {
				Defender struct {
					Species Species
				}
			} `json:"input"`
		}
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			t.Fatal(err)
		}
		s := line.Input.Defender.Species
		if s.Key == "" || seen[s.Key] {
			continue
		}
		seen[s.Key] = true
		out = append(out, s)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(out) != meta.SpeciesCount {
		t.Fatalf("取り出した種族数 = %d, マニフェストの speciesCount = %d", len(out), meta.SpeciesCount)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
