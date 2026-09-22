package judge

import (
	"errors"
	"testing"

	"example.com/pokecalc/engine"
)

// naturePlusSpe / natureMinusSpe は架空の性格(実マスタは使わない)。engine.Nature は
// Plus / Minus の組で1性格を表すため、素早さ以外のどちらかを埋める(値には影響しない)。
var (
	naturePlusSpe  = engine.Nature{Plus: engine.StatSpe, Minus: engine.StatSpA}
	natureMinusSpe = engine.Nature{Plus: engine.StatSpA, Minus: engine.StatSpe}
)

// TestSpeed: 戦闘中の素早さは 実数値(engine.RealStats)→ ランク補正 → こだわりスカーフ の順で求める
// (ADR-0701 §2)。期待値は engine の式からの手計算。
//
//	実数値   = floor((種族値 + 20 + SP) × 性格補正)   ※ Lv50・個体値31 固定
//	ランク   = floor(実数値 × ランク倍率)             ※ +1 は 3/2、-1 は 2/3
//	スカーフ = floor((v × 6144 + 2048 − 1) / 4096)   ※ ×1.5 の五捨五超入
func TestSpeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Individual
		want int
	}{
		{
			// 100 + 20 + 0 = 120(無補正)
			"無振り・無補正",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral},
			120,
		},
		{
			// (100 + 20 + 32) × 1.1 = floor(167.2) = 167
			"最速(SP32・上昇補正)",
			Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}},
			167,
		},
		{
			// 120 × 0.9 = 108
			"下降補正",
			Individual{BaseSpeed: 100, Nature: natureMinusSpe},
			108,
		},
		{
			// 100 + 20 + 32 = 152(無補正・準速)
			"準速(SP32・無補正)",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, SP: engine.Stats{Spe: 32}},
			152,
		},
		{
			// 120 × 3/2 = 180
			"ランク +1",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: 1}},
			180,
		},
		{
			// 120 × 2/3 = 80
			"ランク -1",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: -1}},
			80,
		},
		{
			// 120 × 8/2 = 480
			"ランク +6",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: 6}},
			480,
		},
		{
			// 120 × 2/8 = 30
			"ランク -6",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: -6}},
			30,
		},
		{
			// 120 × 1.5 = 180(割り切れる)
			"こだわりスカーフ",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Scarf: true},
			180,
		},
		{
			// 素早さ以外のランクは素早さに影響しない。
			"素早さ以外のランクは無視",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Atk: 6, Def: -6}},
			120,
		},
		{
			// ランクを先に、スカーフを後に掛ける(ADR-0701 §2)。
			// 120 × 3/2 = 180 → 180 × 1.5 = 270
			"ランク +1 とこだわりスカーフ",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: 1}, Scarf: true},
			270,
		},
		{
			// ADR-0701「テストの期待値」の丸めのケース。
			// 71 + 20 + 0 = 91 → 91 × 1.5 = 136.5 → 五捨五超入で 136(四捨五入なら 137)。
			"こだわりスカーフの五捨五超入(ちょうど .5 は切り捨て)",
			Individual{BaseSpeed: 71, Nature: engine.NatureNeutral, Scarf: true},
			136,
		},
		{
			// 73 + 20 + 0 = 93 → 93 × 1.5 = 139.5 → 139(同上。丸めが 1 か所でないことの確認)。
			"こだわりスカーフの五捨五超入(別の .5)",
			Individual{BaseSpeed: 73, Nature: engine.NatureNeutral, Scarf: true},
			139,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Speed(tt.in)
			if err != nil {
				t.Fatalf("Speed(%+v): %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Speed(%+v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestSpeedRejectsOutOfRangeInput: コアはドメインの不変条件を自分で検証する(coding-rules §3。
// speed-svc の Speed と同じ立場)。範囲外の値で黙って計算を続けない。
func TestSpeedRejectsOutOfRangeInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Individual
		want error
	}{
		{"種族値が 0", Individual{BaseSpeed: 0, Nature: engine.NatureNeutral}, ErrInvalidBaseSpeed},
		{"種族値が負", Individual{BaseSpeed: -1, Nature: engine.NatureNeutral}, ErrInvalidBaseSpeed},
		{"種族値が 255 超", Individual{BaseSpeed: 256, Nature: engine.NatureNeutral}, ErrInvalidBaseSpeed},
		{"SP が負", Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, SP: engine.Stats{Spe: -1}}, ErrInvalidSP},
		{"SP が 32 超", Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, SP: engine.Stats{Spe: 33}}, ErrInvalidSP},
		{
			"素早さ以外の SP も範囲外なら弾く",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, SP: engine.Stats{Atk: 33}},
			ErrInvalidSP,
		},
		{
			// 各ステータスは 32 以下でも合計は 66 以下(CLAUDE.md のドメイン規約)。
			"SP の合計が 66 超",
			Individual{
				BaseSpeed: 100,
				Nature:    engine.NatureNeutral,
				SP:        engine.Stats{HP: 32, Atk: 32, Spe: 32},
			},
			ErrInvalidSP,
		},
		{"ランクが -6 未満", Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: -7}}, ErrInvalidRank},
		{"ランクが +6 超", Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: 7}}, ErrInvalidRank},
		{"素早さ以外のランクも範囲外なら弾く", Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Atk: 7}}, ErrInvalidRank},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Speed(tt.in)
			if !errors.Is(err, tt.want) {
				t.Errorf("Speed(%+v) err = %v, want %v", tt.in, err, tt.want)
			}
			if got != 0 {
				t.Errorf("Speed(%+v) = %d, want 0 on error", tt.in, got)
			}
		})
	}
}

// TestCompareSpeed: outspeeds は厳密な >、speedTie は ==(ADR-0700 §6-1)。
// 同速を真偽値 1 つに丸めない(画面で「抜けている」と区別できなくなるため)。
func TestCompareSpeed(t *testing.T) {
	t.Parallel()

	var (
		fast    = Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}} // 167
		neutral = Individual{BaseSpeed: 100, Nature: engine.NatureNeutral}                     // 120
		scarfed = Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Scarf: true}        // 180
	)

	tests := []struct {
		name               string
		attacker, defender Individual
		want               SpeedComparison
	}{
		{
			"自分の方が速い",
			fast, neutral,
			SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 120, Outspeeds: true, SpeedTie: false},
		},
		{
			"自分の方が遅い(どちらも false)",
			neutral, fast,
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 167, Outspeeds: false, SpeedTie: false},
		},
		{
			"同速(outspeeds は false・speedTie が true)",
			neutral, neutral,
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 120, Outspeeds: false, SpeedTie: true},
		},
		{
			// スカーフが無ければ 120 < 167 で抜けない。スカーフで 180 になり抜ける。
			"こだわりスカーフで抜ける",
			scarfed, fast,
			SpeedComparison{AttackerSpeed: 180, DefenderSpeed: 167, Outspeeds: true, SpeedTie: false},
		},
		{
			"相手のスカーフも考慮する",
			fast, scarfed,
			SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 180, Outspeeds: false, SpeedTie: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CompareSpeed(tt.attacker, tt.defender)
			if err != nil {
				t.Fatalf("CompareSpeed: %v", err)
			}
			if got != tt.want {
				t.Errorf("CompareSpeed = %+v, want %+v", got, tt.want)
			}
			if got.Outspeeds && got.SpeedTie {
				t.Error("outspeeds と speedTie が同時に true になっている")
			}
		})
	}
}

// TestCompareSpeedRejectsOutOfRangeInput: どちらの側が範囲外でも、その sentinel エラーを返す。
func TestCompareSpeedRejectsOutOfRangeInput(t *testing.T) {
	t.Parallel()

	valid := Individual{BaseSpeed: 100, Nature: engine.NatureNeutral}
	badSP := Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, SP: engine.Stats{Spe: 33}}

	for name, sides := range map[string][2]Individual{
		"自分が範囲外": {badSP, valid},
		"相手が範囲外": {valid, badSP},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := CompareSpeed(sides[0], sides[1]); !errors.Is(err, ErrInvalidSP) {
				t.Errorf("err = %v, want ErrInvalidSP", err)
			}
		})
	}
}

// TestIsChoiceScarf: judge は持ち物の一覧を持たず、既知の ID 1 つとの一致だけで
// こだわりスカーフを判定する(ADR-0701 §3)。既定は choicescarf(pokedex-svc の持ち物 ID は
// Showdown の ID 規約 = 小文字英数のみ)。設定で上書きできるのは、実マスタの命名が違っていた場合に
// コードを直さず環境変数 1 行で直せるようにするため。
func TestIsChoiceScarf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		itemID      string
		scarfItemID string
		want        bool
	}{
		{"既定の ID(設定は空)", "choicescarf", "", true},
		{"DefaultChoiceScarfItemID と同じ", DefaultChoiceScarfItemID, "", true},
		{"持ち物なし", "", "", false},
		{"別の持ち物", "test-other-item", "", false},
		{"設定で上書きした ID", "test-scarf", "test-scarf", true},
		{"上書きすると既定は効かない", "choicescarf", "test-scarf", false},
		{"持ち物なしは設定を上書きしても false", "", "test-scarf", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsChoiceScarf(tt.itemID, tt.scarfItemID); got != tt.want {
				t.Errorf("IsChoiceScarf(%q, %q) = %v, want %v", tt.itemID, tt.scarfItemID, got, tt.want)
			}
		})
	}
}

// TestDefaultChoiceScarfItemIDFollowsMasterConvention: 既定値は pokedex-svc の持ち物 ID の
// 命名規則(Showdown の ID: 小文字英数のみ。services/pokedex/importer の toID)に沿っていること。
// 規則から外れた既定値は、実マスタと突き合わせるまで誰も気づけないまま外れ続ける。
func TestDefaultChoiceScarfItemIDFollowsMasterConvention(t *testing.T) {
	t.Parallel()

	if DefaultChoiceScarfItemID == "" {
		t.Fatal("DefaultChoiceScarfItemID が空")
	}
	for _, r := range DefaultChoiceScarfItemID {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			t.Errorf("DefaultChoiceScarfItemID = %q に小文字英数以外の文字 %q がある", DefaultChoiceScarfItemID, r)
		}
	}
}
