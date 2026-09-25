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
		{
			// issue #329: SP 合計はちょうど 66(engine.MaxSPTotal)まで受け付ける境界値。
			// Spe 以外に振った分(Atk:32・HP:2)は各欄 32 以下(MaxSPPerStat)のまま出力に影響しない。
			// 100 + 20 + 32 = 152(無補正)。
			"SP 合計がちょうど 66(境界値・受け付ける)",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, SP: engine.Stats{Spe: 32, Atk: 32, HP: 2}},
			152,
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
		{
			// issue #329: 上の 96 は 66 から遠く、境界(engine.MaxSPTotal+1)を1つずらす退行を検出できない
			// (validateSP の `> engine.MaxSPTotal` を `> engine.MaxSPTotal+1` に変えても検出されなかった)。
			// 各ステータスは 32 以下(MaxSPPerStat の範囲内)のまま合計だけがちょうど 1 超えた 67 で
			// 拒否することをピン留めする(1ステータス検査ではなく合計検査を確実に踏む)。
			"SP の合計がちょうど 67(境界値・拒否する)",
			Individual{
				BaseSpeed: 100,
				Nature:    engine.NatureNeutral,
				SP:        engine.Stats{Spe: 32, Atk: 32, HP: 3},
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

			// JD2 で CompareSpeed に場の効果の引数が増えた(ADR-0702 §4)。SpeedField{} は
			// 「場の効果なし」= JD1 と同じ条件なので、上の表の期待値は 1 つも変えていない。
			got, err := CompareSpeed(tt.attacker, tt.defender, SpeedField{})
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

			if _, err := CompareSpeed(sides[0], sides[1], SpeedField{}); !errors.Is(err, ErrInvalidSP) {
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

// --- JD2: 場の効果(トリックルーム・追い風)。ADR-0702 ---

// TestSpeedTailwind: 追い風は対象側の実数値そのものを ×2 する(ADR-0702 §2・受け入れ条件2)。
// トリックルームと違い、これは値が変わるので Speed() の段で効く。
func TestSpeedTailwind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Individual
		want int
	}{
		{
			// 120 × 2 = 240
			"無振り・無補正 + 追い風",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Tailwind: true},
			240,
		},
		{
			// (100 + 20 + 32) × 1.1 = 167 → × 2 = 334
			"最速 + 追い風",
			Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}, Tailwind: true},
			334,
		},
		{
			// ランクが先、追い風が後: 120 × 3/2 = 180 → × 2 = 360
			"ランク +1 + 追い風",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: 1}, Tailwind: true},
			360,
		},
		{
			// 120 × 2/3 = 80 → × 2 = 160
			"ランク -1 + 追い風",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Ranks: engine.Ranks{Spe: -1}, Tailwind: true},
			160,
		},
		{
			// 71 + 20 + 0 = 91 → × 2 = 182(×2 は整数を保つので丸めは起きない)
			"奇数の実数値 + 追い風",
			Individual{BaseSpeed: 71, Nature: engine.NatureNeutral, Tailwind: true},
			182,
		},
		{
			// 追い風なしは JD1 から変わらない(後方互換。ADR-0702 受け入れ条件5)。
			"追い風なしは JD1 と同じ",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral},
			120,
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

// TestSpeedTailwindAndChoiceScarfChainBeforeRounding: 素早さ補正は 4096 基準で 1 つに連結してから
// **1 回だけ**五捨五超入する(ADR-0702 §2・受け入れ条件3)。出典は @smogon/calc 0.12.0(ADR-0002 が
// 固定した版)の dist/mechanics/util.js の getFinalSpeed: 追い風 8192 と こだわりスカーフ 6144 を
// speedMods に積み、chainMods でまとめてから pokeRound を 1 回掛ける。
//
// 連結後の補正は chainMods([8192, 6144]) = 12288 = ちょうど ×3 になる。
// 各補正ごとに丸める実装(スカーフで丸めてから ×2)は、実数値が奇数のときに 1 ずれる。
func TestSpeedTailwindAndChoiceScarfChainBeforeRounding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Individual
		want int
		// naive は「スカーフを先に五捨五超入してから追い風で ×2」した場合の誤った値。
		// want と違うことを明示して、丸めの向き・回数が実装から落ちたら気づけるようにする。
		naive int
	}{
		{
			// 71 + 20 + 0 = 91(奇数)。91 × 3 = 273。
			// スカーフ先: 91 × 1.5 = 136.5 → 五捨五超入で 136 → × 2 = 272(ずれる)。
			"実数値 91 + スカーフ + 追い風",
			Individual{BaseSpeed: 71, Nature: engine.NatureNeutral, Scarf: true, Tailwind: true},
			273, 272,
		},
		{
			// 73 + 20 + 0 = 93(奇数)。93 × 3 = 279。スカーフ先だと 139 × 2 = 278。
			"実数値 93 + スカーフ + 追い風",
			Individual{BaseSpeed: 73, Nature: engine.NatureNeutral, Scarf: true, Tailwind: true},
			279, 278,
		},
		{
			// (100 + 20 + 32) × 1.1 = 167(奇数)。167 × 3 = 501。
			// スカーフ先: 167 × 1.5 = 250.5 → 250 → × 2 = 500。
			"実数値 167 + スカーフ + 追い風",
			Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}, Scarf: true, Tailwind: true},
			501, 500,
		},
		{
			// 実数値が偶数なら連結でも逐次でも同じ(120 × 3 = 360)。
			// 「奇数のときだけずれる」ことを示すための対照。
			"実数値 120 + スカーフ + 追い風(偶数なので差は出ない)",
			Individual{BaseSpeed: 100, Nature: engine.NatureNeutral, Scarf: true, Tailwind: true},
			360, 360,
		},
		{
			// ランク → 補正の連結 の順は変わらない。120 × 3/2 = 180 → × 3 = 540。
			"ランク +1 + スカーフ + 追い風",
			Individual{
				BaseSpeed: 100, Nature: engine.NatureNeutral,
				Ranks: engine.Ranks{Spe: 1}, Scarf: true, Tailwind: true,
			},
			540, 540,
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
			if tt.naive != tt.want && got == tt.naive {
				t.Errorf("Speed(%+v) = %d は補正ごとに丸めた値。連結してから 1 回だけ五捨五超入する(ADR-0702 §2)",
					tt.in, got)
			}
		})
	}
}

// TestSpeedSingleModifiersUnchangedFromJD1: 補正が 1 つだけのときの値は JD1 から変わらない
// (ADR-0702 受け入れ条件3)。連結の導入でスカーフ単独の丸めが動いていないことを固定する。
func TestSpeedSingleModifiersUnchangedFromJD1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   Individual
		want int
	}{
		// 91 × 1.5 = 136.5 → 五捨五超入で 136(ADR-0701「テストの期待値」と同じ値)。
		{"スカーフ単独(.5 は切り捨て)", Individual{BaseSpeed: 71, Nature: engine.NatureNeutral, Scarf: true}, 136},
		{"スカーフ単独(別の .5)", Individual{BaseSpeed: 73, Nature: engine.NatureNeutral, Scarf: true}, 139},
		{
			"スカーフ単独(167 → 250.5 → 250)",
			Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}, Scarf: true},
			250,
		},
		// 追い風単独は ×2 ちょうどなので丸めが起きない。
		{"追い風単独", Individual{BaseSpeed: 71, Nature: engine.NatureNeutral, Tailwind: true}, 182},
		{"補正なし", Individual{BaseSpeed: 71, Nature: engine.NatureNeutral}, 91},
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

// TestCompareSpeedTrickRoom: トリックルームは実数値を変えず、outspeeds(= 自分が先に動くか)の
// 比較の向きだけを反転する(ADR-0702 §3・受け入れ条件4)。speedTie は反転しない。
func TestCompareSpeedTrickRoom(t *testing.T) {
	t.Parallel()

	var (
		fast    = Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}} // 167
		neutral = Individual{BaseSpeed: 100, Nature: engine.NatureNeutral}                     // 120
	)

	tests := []struct {
		name               string
		attacker, defender Individual
		field              SpeedField
		want               SpeedComparison
	}{
		{
			"トリックルーム無し: 速い方が先に動く",
			fast, neutral, SpeedField{},
			SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 120, Outspeeds: true, SpeedTie: false},
		},
		{
			// 実数値は変わらない。反転するのは outspeeds だけ。
			"トリックルーム中: 速い方が後になる",
			fast, neutral, SpeedField{TrickRoom: true},
			SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 120, Outspeeds: false, SpeedTie: false},
		},
		{
			"トリックルーム無し: 遅いと抜けない",
			neutral, fast, SpeedField{},
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 167, Outspeeds: false, SpeedTie: false},
		},
		{
			"トリックルーム中: 遅い方が先に動く",
			neutral, fast, SpeedField{TrickRoom: true},
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 167, Outspeeds: true, SpeedTie: false},
		},
		{
			"同速はトリックルーム無しでも outspeeds=false・speedTie=true",
			neutral, neutral, SpeedField{},
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 120, Outspeeds: false, SpeedTie: true},
		},
		{
			// 同速はゲームでもトリックルームの有無に関わらず行動順が決まらない(ADR-0702 §3)。
			"同速はトリックルーム中でも反転しない",
			neutral, neutral, SpeedField{TrickRoom: true},
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 120, Outspeeds: false, SpeedTie: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CompareSpeed(tt.attacker, tt.defender, tt.field)
			if err != nil {
				t.Fatalf("CompareSpeed: %v", err)
			}
			if got != tt.want {
				t.Errorf("CompareSpeed(field=%+v) = %+v, want %+v", tt.field, got, tt.want)
			}
			if got.Outspeeds && got.SpeedTie {
				t.Error("outspeeds と speedTie が同時に true になっている")
			}
		})
	}
}

// TestCompareSpeedTailwindPerSide: 追い風は指定した側だけに乗る(ADR-0702 受け入れ条件2)。
// 片側だけ ×2 になることと、トリックルームと組み合わせても向きの反転だけが効くことを見る。
func TestCompareSpeedTailwindPerSide(t *testing.T) {
	t.Parallel()

	var (
		fast    = Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}} // 167
		neutral = Individual{BaseSpeed: 100, Nature: engine.NatureNeutral}                     // 120
	)
	withTailwind := func(in Individual) Individual {
		in.Tailwind = true
		return in
	}

	tests := []struct {
		name               string
		attacker, defender Individual
		field              SpeedField
		want               SpeedComparison
	}{
		{
			// 120 × 2 = 240 > 167
			"自分だけ追い風で抜ける",
			withTailwind(neutral), fast, SpeedField{},
			SpeedComparison{AttackerSpeed: 240, DefenderSpeed: 167, Outspeeds: true, SpeedTie: false},
		},
		{
			// 相手だけ ×2: 167 × 2 = 334
			"相手だけ追い風で抜き返される",
			neutral, withTailwind(fast), SpeedField{},
			SpeedComparison{AttackerSpeed: 120, DefenderSpeed: 334, Outspeeds: false, SpeedTie: false},
		},
		{
			// 両方 ×2 なら大小関係は変わらない(240 < 334)。
			"両方追い風なら関係は変わらない",
			withTailwind(neutral), withTailwind(fast), SpeedField{},
			SpeedComparison{AttackerSpeed: 240, DefenderSpeed: 334, Outspeeds: false, SpeedTie: false},
		},
		{
			// 追い風で同速になることもある(120 × 2 = 240 = 240)。
			"追い風で同速になる",
			withTailwind(neutral), Individual{BaseSpeed: 220, Nature: engine.NatureNeutral}, SpeedField{},
			SpeedComparison{AttackerSpeed: 240, DefenderSpeed: 240, Outspeeds: false, SpeedTie: true},
		},
		{
			// 追い風で上回った側が、トリックルーム中は後攻になる。
			"追い風 + トリックルーム",
			withTailwind(neutral), fast, SpeedField{TrickRoom: true},
			SpeedComparison{AttackerSpeed: 240, DefenderSpeed: 167, Outspeeds: false, SpeedTie: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := CompareSpeed(tt.attacker, tt.defender, tt.field)
			if err != nil {
				t.Fatalf("CompareSpeed: %v", err)
			}
			if got != tt.want {
				t.Errorf("CompareSpeed(field=%+v) = %+v, want %+v", tt.field, got, tt.want)
			}
			if got.Outspeeds && got.SpeedTie {
				t.Error("outspeeds と speedTie が同時に true になっている")
			}
		})
	}
}

// TestCompareSpeedZeroSpeedFieldMatchesJD1: SpeedField のゼロ値は「場の効果なし」で、JD1 の
// 挙動と完全に一致する(ADR-0702 受け入れ条件5 の後方互換をコアの側で固定する)。
func TestCompareSpeedZeroSpeedFieldMatchesJD1(t *testing.T) {
	t.Parallel()

	attacker := Individual{BaseSpeed: 100, Nature: naturePlusSpe, SP: engine.Stats{Spe: 32}}
	defender := Individual{BaseSpeed: 100, Nature: engine.NatureNeutral}

	got, err := CompareSpeed(attacker, defender, SpeedField{})
	if err != nil {
		t.Fatalf("CompareSpeed: %v", err)
	}
	want := SpeedComparison{AttackerSpeed: 167, DefenderSpeed: 120, Outspeeds: true, SpeedTie: false}
	if got != want {
		t.Errorf("CompareSpeed(SpeedField{}) = %+v, want %+v(JD1 と同じ)", got, want)
	}
}
