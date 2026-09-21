package engine

// ゴールデン fixture(testdata/golden)を読むときの共通の約束。ビルドタグなし:
// ゴールデン(-tags golden)と全種族テスト(-tags allspecies)の両方から使い、
// 読み込み規則そのものは make test で固定する。
//
// P2-1b(ADR-0002 §決定 5 の追記「ゴールデンの oracle を Champions へ切り替え」)で導入。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// ゴールデンの種族集合(@smogon/calc 0.12.0 の Champions 世代。内部フォーム除外)の件数の範囲。
//
// 件数は再生成とレギュレーション更新で動くため固定値では守らない。代わりに次の取り違えを検出できる範囲で守る:
//   - 下限 300: 基底種だけ(231)に絞った、フォーム/メガを落とした、フィルタで空になった
//     (P1-6 レビューの改善要望: 0 でないことだけでは足りない)。
//     参考: P2-1b 調査時点で 358(calc 359 から内部フォーム Aegislash-Both を除く)。
//     レギュレーションは累積で集合が増える(ADR-0002 §決定 4)ので、下限を割るのは誤りのサイン。
//   - 上限 1000: gen9 の参考集合(CAP 等を含む 1392)に戻ってしまった。
//
// 上限を超える正当な理由(Champions の集合が大きく増えた)が出たら、ADR-0002 を更新してから変える。
const (
	goldenMinSpeciesCount = 300
	goldenMaxSpeciesCount = 1000
)

// decodeStrictGolden は fixture の JSON を、未知のフィールドを拒否して dst に読む。
//
// engine の型(DamageInput / Individual)の多くは JSON タグを持たず、encoding/json は
// 大文字小文字を無視してフィールド名で対応づける。フィールドの改名や生成器側の綴り違いがあると、
// 既定の Unmarshal は値を黙ってゼロ値のままにし、テストは「別の入力」で照合してしまう。
// それを読み込み時点で落とす(P1-6 レビューの改善要望)。
func decodeStrictGolden(raw []byte, dst any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return errors.New("空の JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// 1値のあとに余分なデータがあれば壊れている(連結・切り詰めの検出)。
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("JSON 値のあとに余分なデータがある")
	}
	return nil
}

// minimalGoldenInput は fixture の input と同じ形の最小の DamageInput(testdata/golden/fixed.json の形)。
const minimalGoldenInput = `{
  "Format": "single",
  "Attacker": {"Species": {"Key": "a", "Types": ["fire"], "BaseStats": {"hp": 80, "atk": 80, "def": 80, "spa": 80, "spd": 80, "spe": 80}},
    "Level": 50, "Nature": {}, "SP": {"hp": 0, "atk": 0, "def": 0, "spa": 0, "spd": 0, "spe": 0}, "Ranks": {},
    "Status": "none", "Ability": {"ID": "", "Effect": null}, "Item": {"ID": "x", "Effect": {"StatMods": {"atk": 6144}}}},
  "Defender": {"Species": {"Key": "d", "Types": ["grass"], "BaseStats": {"hp": 80, "atk": 80, "def": 80, "spa": 80, "spd": 80, "spe": 80}},
    "Level": 50, "Nature": {"Plus": "def", "Minus": "atk"}, "SP": {"hp": 32, "atk": 0, "def": 32, "spa": 0, "spd": 0, "spe": 0}, "Ranks": {"def": 1},
    "Status": "none", "Ability": {"ID": "", "Effect": null}, "Item": null},
  "Move": {"ID": "m", "Type": "fire", "Category": "special", "Power": 90, "Priority": 0},
  "Field": {"Weather": "none", "Terrain": "none", "DefenderScreens": {"Reflect": false, "LightScreen": false, "AuroraVeil": false}},
  "Critical": false
}`

func TestDecodeStrictGoldenRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr string // 空なら成功を期待
	}{
		{name: "fixture と同じ形は読める", mutate: func(s string) string { return s }},
		{name: "トップレベルの綴り違い", mutate: func(s string) string {
			return strings.Replace(s, `"Critical": false`, `"Critcal": true`, 1)
		}, wantErr: "Critcal"},
		{name: "個体のフィールド改名(SP が黙ってゼロになるのを防ぐ)", mutate: func(s string) string {
			return strings.Replace(s, `"SP": {"hp": 32`, `"EVs": {"hp": 32`, 1)
		}, wantErr: "EVs"},
		{name: "ステータスのキーの綴り違い", mutate: func(s string) string {
			return strings.Replace(s, `"SP": {"hp": 32, "atk": 0, "def": 32`, `"SP": {"hp": 32, "atk": 0, "dfe": 32`, 1)
		}, wantErr: "dfe"},
		{name: "持ち物の補正定義の未知フィールド", mutate: func(s string) string {
			return strings.Replace(s, `{"StatMods": {"atk": 6144}}`, `{"StatMods": {"atk": 6144}, "PowerModz": 4915}`, 1)
		}, wantErr: "PowerModz"},
		{name: "場のフィールド改名", mutate: func(s string) string {
			return strings.Replace(s, `"Weather": "none"`, `"Weathr": "rain"`, 1)
		}, wantErr: "Weathr"},
		{name: "値のあとの余分なデータ", mutate: func(s string) string { return s + ` {}` }, wantErr: "余分"},
		{name: "空", mutate: func(string) string { return "  " }, wantErr: "空"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in DamageInput
			err := decodeStrictGolden([]byte(tt.mutate(minimalGoldenInput)), &in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("読めるはずの fixture 形式を拒否した: %v", err)
				}
				// 大文字小文字を無視した対応づけで、値が実際に入っていること。
				if in.Defender.SP.HP != 32 || in.Defender.SP.Def != 32 || in.Defender.Nature.Plus != StatDef ||
					in.Attacker.Item == nil || in.Move.Power != 90 || in.Format != "single" {
					t.Fatalf("値が入っていない: %+v", in)
				}
				return
			}
			if err == nil {
				t.Fatalf("未知フィールド/壊れた JSON を受け入れた(値が黙ってゼロ値になる)")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("エラーが原因の名前を含まない: %v(want %q を含む)", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeStrictGoldenIndividual(t *testing.T) {
	ok := `{"Species":{"Key":"a","Types":["fire"],"BaseStats":{"hp":80,"atk":80,"def":80,"spa":80,"spd":80,"spe":80}},"Level":50,"Nature":{},"SP":{"spe":32},"Ranks":{},"Status":"none","Ability":{"ID":"","Effect":null},"Item":null}`
	var v Individual
	if err := decodeStrictGolden([]byte(ok), &v); err != nil {
		t.Fatalf("stats-species の individual の形を拒否した: %v", err)
	}
	if v.SP.Spe != 32 {
		t.Fatalf("SP.Spe=%d want 32", v.SP.Spe)
	}
	bad := strings.Replace(ok, `"SP":{"spe":32}`, `"SP":{"speed":32}`, 1)
	if err := decodeStrictGolden([]byte(bad), &v); err == nil || !strings.Contains(err.Error(), "speed") {
		t.Fatalf("未知のステータスキーを受け入れた: err=%v", err)
	}
}
