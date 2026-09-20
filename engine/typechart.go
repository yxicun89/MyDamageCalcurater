package engine

// タイプ相性表(第6世代以降)。
// 単一マッチアップの倍率を「×2 した整数コード」で持つ:
//   0 = 無効(×0)、1 = いまひとつ(×0.5)、2 = 等倍(×1.0)、4 = 抜群(×2.0)
// 表に無い組合せは等倍(2)。

type typeRelation struct {
	Double []Type // 抜群(コード4)
	Half   []Type // いまひとつ(コード1)
	Zero   []Type // 無効(コード0)
}

var typeChartRaw = map[Type]typeRelation{
	TypeNormal:   {Half: []Type{TypeRock, TypeSteel}, Zero: []Type{TypeGhost}},
	TypeFire:     {Double: []Type{TypeGrass, TypeIce, TypeBug, TypeSteel}, Half: []Type{TypeFire, TypeWater, TypeRock, TypeDragon}},
	TypeWater:    {Double: []Type{TypeFire, TypeGround, TypeRock}, Half: []Type{TypeWater, TypeGrass, TypeDragon}},
	TypeElectric: {Double: []Type{TypeWater, TypeFlying}, Half: []Type{TypeElectric, TypeGrass, TypeDragon}, Zero: []Type{TypeGround}},
	TypeGrass:    {Double: []Type{TypeWater, TypeGround, TypeRock}, Half: []Type{TypeFire, TypeGrass, TypePoison, TypeFlying, TypeBug, TypeDragon, TypeSteel}},
	TypeIce:      {Double: []Type{TypeGrass, TypeGround, TypeFlying, TypeDragon}, Half: []Type{TypeFire, TypeWater, TypeIce, TypeSteel}},
	TypeFighting: {Double: []Type{TypeNormal, TypeIce, TypeRock, TypeDark, TypeSteel}, Half: []Type{TypePoison, TypeFlying, TypePsychic, TypeBug, TypeFairy}, Zero: []Type{TypeGhost}},
	TypePoison:   {Double: []Type{TypeGrass, TypeFairy}, Half: []Type{TypePoison, TypeGround, TypeRock, TypeGhost}, Zero: []Type{TypeSteel}},
	TypeGround:   {Double: []Type{TypeFire, TypeElectric, TypePoison, TypeRock, TypeSteel}, Half: []Type{TypeGrass, TypeBug}, Zero: []Type{TypeFlying}},
	TypeFlying:   {Double: []Type{TypeGrass, TypeFighting, TypeBug}, Half: []Type{TypeElectric, TypeRock, TypeSteel}},
	TypePsychic:  {Double: []Type{TypeFighting, TypePoison}, Half: []Type{TypePsychic, TypeSteel}, Zero: []Type{TypeDark}},
	TypeBug:      {Double: []Type{TypeGrass, TypePsychic, TypeDark}, Half: []Type{TypeFire, TypeFighting, TypePoison, TypeFlying, TypeGhost, TypeSteel, TypeFairy}},
	TypeRock:     {Double: []Type{TypeFire, TypeIce, TypeFlying, TypeBug}, Half: []Type{TypeFighting, TypeGround, TypeSteel}},
	TypeGhost:    {Double: []Type{TypePsychic, TypeGhost}, Half: []Type{TypeDark}, Zero: []Type{TypeNormal}},
	TypeDragon:   {Double: []Type{TypeDragon}, Half: []Type{TypeSteel}, Zero: []Type{TypeFairy}},
	TypeDark:     {Double: []Type{TypePsychic, TypeGhost}, Half: []Type{TypeFighting, TypeDark, TypeFairy}},
	TypeSteel:    {Double: []Type{TypeIce, TypeRock, TypeFairy}, Half: []Type{TypeFire, TypeWater, TypeElectric, TypeSteel}},
	TypeFairy:    {Double: []Type{TypeFighting, TypeDragon, TypeDark}, Half: []Type{TypeFire, TypePoison, TypeSteel}},
}

// typeChart[攻撃タイプ][防御タイプ] = 倍率コード(0/1/2/4)。等倍は 2。
var typeChart = buildTypeChart()

func buildTypeChart() map[Type]map[Type]int {
	m := make(map[Type]map[Type]int, len(typeChartRaw))
	for atk, rel := range typeChartRaw {
		row := make(map[Type]int)
		for _, d := range rel.Double {
			row[d] = 4
		}
		for _, h := range rel.Half {
			row[h] = 1
		}
		for _, z := range rel.Zero {
			row[z] = 0
		}
		m[atk] = row
	}
	return m
}

// typeCode は攻撃タイプ atk が防御タイプ def に対して持つ倍率コード(0/1/2/4)を返す。
func typeCode(atk, def Type) int {
	if atk == TypeNone || def == TypeNone {
		return 2 // 等倍
	}
	if row, ok := typeChart[atk]; ok {
		if c, ok := row[def]; ok {
			return c
		}
	}
	return 2
}

// TypeEffectiveness は攻撃タイプと防御側タイプ列に対する相性を返す。
// num/den で厳密な倍率、mult は表示用の float(0/0.25/0.5/1/2/4)。
func TypeEffectiveness(atk Type, defTypes []Type) (num, den int, mult float64) {
	num, den = 1, 1
	for _, dt := range defTypes {
		if dt == TypeNone {
			continue
		}
		num *= typeCode(atk, dt)
		den *= 2
	}
	if den == 1 {
		// タイプなし防御 → 等倍
		return 1, 1, 1.0
	}
	return num, den, float64(num) / float64(den)
}
