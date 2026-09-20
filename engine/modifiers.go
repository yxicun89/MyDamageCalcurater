package engine

// 補正(天候・フィールド・壁・持ち物・特性)。
//
// 持ち物・特性の補正定義は「マスタから解決した ItemEffect / AbilityEffect」を
// engine が受け取って適用する(CLAUDE.md: 持ち物・特性をコードにハードコードしない)。
// 天候・フィールド・壁はゲーム機構なので engine のルールとして実装する。
//
// 適用位置(ADR-0004):
//   - 天候のダメージ倍率: base 段階で個別に pokeRound
//   - 天候の防御実数値補正(すなあらし・ゆき): 実数値段階で chainMods
//   - 持ち物の実数値補正(こだわり系・とつげきチョッキ等): 実数値段階で chainMods
//   - フィールド・壁・持ち物/特性のダメージ倍率: やけどの後に chainMods で1回

// ItemEffect はダメージに影響する持ち物の補正(4096基準)。
type ItemEffect struct {
	StatMods           map[StatKey]int // 実数値倍率。例 こだわりハチマキ{atk:6144}, とつげきチョッキ{spd:6144}, しんかのきせき{def:6144,spd:6144}
	DamageMod          int             // ダメージ倍率。例 いのちのたま5324、ちからのハチマキ4505。0 は補正なし
	OnlySuperEffective bool            // たつじんのおび: 抜群時のみ DamageMod を適用
	BoostType          Type            // タイプ強化(もくたん等)対象タイプ
	BoostTypeMod       int             // 例 4915(=約1.2倍)
	ResistBerryType    Type            // 半減きのみ: このタイプの抜群技を半減(防御側)
}

// AbilityEffect はダメージに影響する特性の補正(4096基準)。
type AbilityEffect struct {
	StabMod              int          // てきおうりょく: 8192。0 は通常(6144)
	OffBoostType         Type         // 攻撃側タイプ強化の対象タイプ
	OffBoostTypeMod      int          // 例 6144
	DefResistType        map[Type]int // 被ダメ軽減。例 あついしぼう{fire:2048, ice:2048}
	ReduceSuperEffective int          // ハードロック/フィルター等: 抜群時に軽減(例 3072)
	IgnoresBurn          bool         // こんじょう等: やけどの攻撃半減を無効化
}

func hasType(in Individual, t Type) bool {
	for _, ty := range in.Species.Types {
		if ty == t {
			return true
		}
	}
	return false
}

// weatherDamageMod は天候による技ダメージ倍率(base段階)を返す。
func weatherDamageMod(w Weather, moveType Type) int {
	switch w {
	case WeatherSun:
		if moveType == TypeFire {
			return 6144 // ×1.5
		}
		if moveType == TypeWater {
			return 2048 // ×0.5
		}
	case WeatherRain:
		if moveType == TypeWater {
			return 6144
		}
		if moveType == TypeFire {
			return 2048
		}
	}
	return Modifier4096
}

// terrainDamageMod はフィールドによる技ダメージ倍率を返す(接地している前提)。
func terrainDamageMod(terr Terrain, moveType Type) int {
	switch terr {
	case TerrainElectric:
		if moveType == TypeElectric {
			return 5325 // ×1.3
		}
	case TerrainGrassy:
		if moveType == TypeGrass {
			return 5325
		}
	case TerrainPsychic:
		if moveType == TypePsychic {
			return 5325
		}
	case TerrainMisty:
		if moveType == TypeDragon {
			return 2048 // ×0.5
		}
	}
	return Modifier4096
}

// screenDamageMod は壁による軽減倍率を返す。急所は壁を貫通するため呼び出し側で除外する。
// シングルは 2048(×0.5)。
func screenDamageMod(in DamageInput) int {
	s := in.Field.DefenderScreens
	switch in.Move.Category {
	case CategoryPhysical:
		if s.Reflect || s.AuroraVeil {
			return 2048
		}
	case CategorySpecial:
		if s.LightScreen || s.AuroraVeil {
			return 2048
		}
	}
	return Modifier4096
}

// offensiveStatMod は攻撃側の持ち物による攻撃実数値の倍率(chainMods 済み)を返す。
func offensiveStatMod(in DamageInput, atkKey StatKey) int {
	var mods []int
	if e := itemEffect(in.Attacker.Item); e != nil {
		if m, ok := e.StatMods[atkKey]; ok {
			mods = append(mods, m)
		}
	}
	return chainMods(mods)
}

// defensiveStatMod は防御側の持ち物・天候による防御実数値の倍率(chainMods 済み)を返す。
func defensiveStatMod(in DamageInput, defKey StatKey) int {
	var mods []int
	if e := itemEffect(in.Defender.Item); e != nil {
		if m, ok := e.StatMods[defKey]; ok {
			mods = append(mods, m)
		}
	}
	// すなあらし: いわタイプの特防 ×1.5 / ゆき: こおりタイプの防御 ×1.5
	if in.Field.Weather == WeatherSand && defKey == StatSpD && hasType(in.Defender, TypeRock) {
		mods = append(mods, 6144)
	}
	if in.Field.Weather == WeatherSnow && defKey == StatDef && hasType(in.Defender, TypeIce) {
		mods = append(mods, 6144)
	}
	return chainMods(mods)
}

func itemEffect(i *Item) *ItemEffect {
	if i == nil {
		return nil
	}
	return i.Effect
}

// otherModifiers はやけどの後に chainMods で1回適用する「その他補正」の一覧を返す。
// フィールド・壁・持ち物ダメージ倍率・特性を含む。
func otherModifiers(in DamageInput) []int {
	var mods []int
	_, _, mult := TypeEffectiveness(in.Move.Type, in.Defender.Species.Types)
	superEffective := mult > 1

	if tm := terrainDamageMod(in.Field.Terrain, in.Move.Type); tm != Modifier4096 {
		mods = append(mods, tm)
	}
	if !in.Critical {
		if sm := screenDamageMod(in); sm != Modifier4096 {
			mods = append(mods, sm)
		}
	}
	// 攻撃側の持ち物
	if e := itemEffect(in.Attacker.Item); e != nil {
		if e.BoostType != TypeNone && e.BoostType == in.Move.Type && e.BoostTypeMod != 0 {
			mods = append(mods, e.BoostTypeMod)
		}
		if e.DamageMod != 0 && (!e.OnlySuperEffective || superEffective) {
			mods = append(mods, e.DamageMod)
		}
	}
	// 攻撃側の特性(タイプ強化)
	if ae := in.Attacker.Ability.Effect; ae != nil {
		if ae.OffBoostType != TypeNone && ae.OffBoostType == in.Move.Type && ae.OffBoostTypeMod != 0 {
			mods = append(mods, ae.OffBoostTypeMod)
		}
	}
	// 防御側の持ち物(半減きのみ)
	if e := itemEffect(in.Defender.Item); e != nil {
		if e.ResistBerryType != TypeNone && e.ResistBerryType == in.Move.Type && superEffective {
			mods = append(mods, 2048)
		}
	}
	// 防御側の特性(タイプ軽減・抜群軽減)
	if de := in.Defender.Ability.Effect; de != nil {
		if m, ok := de.DefResistType[in.Move.Type]; ok {
			mods = append(mods, m)
		}
		if de.ReduceSuperEffective != 0 && superEffective {
			mods = append(mods, de.ReduceSuperEffective)
		}
	}
	return mods
}
