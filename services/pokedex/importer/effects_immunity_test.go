package importer_test

// P2-3b / ADR-0106 §決定 5: コミットされた本番の効果定義 `data/importer/effects.json` に
// 無効・吸収の特性が入っていること、そして照合の設定 `data/importer/config.json` の
// reconcile.effectHooks が無効・吸収のハンドラを含むこと。
//
// effectHooks を足さないと、新しい定義がすべて `effect-no-hook` 警告になる(ADR-0103 §6)。
// Showdown の実装で、無効・吸収の特性は onTryHit(ちょすい・もらいび・そうしょく等)と
// onImmunity(ふゆう)を使う。倍率系のハンドラ(onModifyAtk 等)しか見ていない今の一覧では拾えない。
//
// 実データ(data/generated)は読まない。読むのは Git にコミットされた英語 ID と整数だけ。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"example.com/pokecalc/engine"
	"example.com/pokecalc/services/internal/master"
	"example.com/pokecalc/services/pokedex/importer"
)

// repoEffectsPath / repoConfigPath はコミットされた本番の定義。
func repoDataPath(name string) string {
	return filepath.Join("..", "..", "..", "data", "importer", name)
}

// repoImmunityAbilities は data/importer/effects.json に入っているべき特性(英語 ID = toID)と、
// その特性が 0 にする攻撃タイプ。ゴールデンで oracle と照合した8件(ADR-0106 §決定 5・8)。
// Dry Skin と Storm Drain は入れない(ADR-0106 §限界 1・2)。
var repoImmunityAbilities = map[string]engine.Type{
	"levitate":     engine.TypeGround,
	"waterabsorb":  engine.TypeWater,
	"voltabsorb":   engine.TypeElectric,
	"eartheater":   engine.TypeGround,
	"flashfire":    engine.TypeFire,
	"sapsipper":    engine.TypeGrass,
	"motordrive":   engine.TypeElectric,
	"lightningrod": engine.TypeElectric,
}

// repoAbsorbAbilities は吸収(absorb)として持つべき特性。残りは無効(immune)。
var repoAbsorbAbilities = map[string]bool{
	"waterabsorb": true, "voltabsorb": true, "eartheater": true,
	"flashfire": true, "sapsipper": true, "motordrive": true, "lightningrod": true,
}

// fullTypeChart は 18 タイプすべてを持つ相性表(効果定義のタイプ検証にだけ使う)。
func fullTypeChart(t *testing.T) engine.TypeChart {
	t.Helper()
	ids := []string{"normal", "fire", "water", "electric", "grass", "ice", "fighting", "poison",
		"ground", "flying", "psychic", "bug", "rock", "ghost", "dragon", "dark", "steel", "fairy"}
	rows := make([]master.TypeRow, 0, len(ids))
	for i, id := range ids {
		rows = append(rows, master.TypeRow{ID: id, SortOrder: i + 1, NameJa: "テスト" + id})
	}
	c, err := master.TypeChart(rows, nil)
	if err != nil {
		t.Fatalf("TypeChart: %v", err)
	}
	return c
}

func TestRepoEffectsHaveTypeImmunityAndAbsorption(t *testing.T) {
	raw, err := os.ReadFile(repoDataPath("effects.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		SchemaVersion int                        `json:"schemaVersion"`
		Abilities     map[string]json.RawMessage `json:"abilities"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("data/importer/effects.json: %v", err)
	}
	chart := fullTypeChart(t)

	for id, blocked := range repoImmunityAbilities {
		def, ok := file.Abilities[id]
		if !ok {
			t.Errorf("data/importer/effects.json に %q が無い(ADR-0106 §決定 5)", id)
			continue
		}
		eff, err := master.DecodeAbilityEffect(def, chart)
		if err != nil {
			t.Errorf("%s の定義を読めない: %v", id, err)
			continue
		}
		immune := false
		for _, ty := range eff.DefImmuneTypes {
			if ty == blocked {
				immune = true
			}
		}
		_, absorb := eff.DefAbsorbTypes[blocked]
		switch {
		case repoAbsorbAbilities[id] && !absorb:
			t.Errorf("%s は %s を吸収(DefAbsorbTypes)にする: %+v", id, blocked, eff)
		case !repoAbsorbAbilities[id] && !immune:
			t.Errorf("%s は %s を無効(DefImmuneTypes)にする: %+v", id, blocked, eff)
		}
		if immune && absorb {
			t.Errorf("%s が %s を無効と吸収の両方にしている", id, blocked)
		}
	}
}

// 照合の設定が無効・吸収のハンドラを見ていること(ADR-0106 §決定 5・人間の確認事項 2)。
func TestRepoConfigEffectHooksCoverImmunity(t *testing.T) {
	raw, err := os.ReadFile(repoDataPath("config.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := importer.DecodeConfig(raw)
	if err != nil {
		t.Fatalf("data/importer/config.json: %v", err)
	}
	if cfg.Reconcile == nil {
		t.Fatal("config.json に reconcile が無い")
	}
	have := map[string]bool{}
	for _, h := range cfg.Reconcile.EffectHooks {
		have[h] = true
	}
	for _, want := range []string{"onTryHit", "onImmunity"} {
		if !have[want] {
			t.Errorf("reconcile.effectHooks に %q が無い(無効・吸収の定義が全部 effect-no-hook 警告になる。ADR-0103 §6)", want)
		}
	}
}
