//go:build allspecies

package engine

// 逆算(P1-8)の Recall@5 合格基準。docs/test-strategy.md「逆算(調整推定)のテスト」:
//
//	1. 既知の調整(SP配分・性格・持ち物)でダメージを生成
//	2. 乱数の1段階を選び、ゲーム内表示と同じ丸め(HP%)をかけて観測値にする
//	3. 逆算にかけ、正解が上位5候補に入る割合(Recall@5)を測る
//	- 合格基準: 1回観測で Recall@5 >= 80%、2回観測で >= 95%
//	- 全ポケモンからランダムに 1,000 ケース(固定シード)
//
// **この基準は緩めない**(CLAUDE.md 絶対ルール6)。落ちたら逆算の順序規則(ADR-0010 §6.3)を
// 直すのであって、しきい値・シード・ケース数を動かして通してはならない。
//
// 全種族 × 3,267 格子点 × 持ち物2 で約1分かかるため、既存の全種族テストと同じ
// allspecies タグに置く(make test-all-species / go test -tags allspecies -run AllSpecies)。
// make test 側の早期検知は engine/reverse_test.go の TestReverseRecallSmoke(基準ではない)。

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

// TestAllSpeciesReverseRecall は全種族・固定シード 1,000 ケースで Recall@5 を測る。
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
		t.Run(fmt.Sprintf("%s/観測%d件", tt.side, tt.nObs), func(t *testing.T) {
			hit, total := reverseRecall(t, tt.side, species, reverseRecallCases, tt.nObs, reverseRecallSeed)
			if total != reverseRecallCases {
				t.Fatalf("ケース数 = %d, want %d", total, reverseRecallCases)
			}
			got := 100 * hit / total
			t.Logf("side=%s 観測 %d 件: Recall@5 = %d%% (%d/%d)", tt.side, tt.nObs, got, hit, total)
			if got < tt.want {
				t.Errorf("side=%s 観測 %d 件の Recall@5 = %d%% (%d/%d), want >= %d%%"+
					"(しきい値・シード・ケース数を変えて通さないこと)",
					tt.side, tt.nObs, got, hit, total, tt.want)
			}
		})
	}
}

// loadAllSpeciesForRecall は testdata/golden の防御側網羅ベクタから全種族を取り出す。
// 逆算用に別の種族表を作らず、ゴールデンと同じ母集団を使う(ADR-0010 §7)。
// Key 昇順に並べ、固定シードでの抽出を決定的にする。
func loadAllSpeciesForRecall(t *testing.T) []Species {
	t.Helper()
	const dir = "../testdata/golden"
	const name = "defense-species.jsonl.gz"

	// マニフェストの SHA256 と突き合わせ、母集団が差し替わっていないことを確かめる。
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
