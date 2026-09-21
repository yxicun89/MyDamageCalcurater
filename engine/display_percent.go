package engine

import "math"

// 表示%(アプリが画面に出すダメージ%・確率)。定義は ADR-0010 §3.2〜§3.4。
//
// 観測%(ObservedPercent。逆算の入力・整数%)とは別概念。ここの値は 0.1% 単位の整数
// (tenths)で、734 は 73.4% を表す。float でダメージ%を近似しないため、小数への変換は
// 境界(engine/wasmapi)だけが行う。

// DisplayPercentTenthsFloor はダメージを HP に対する割合(0.1% 単位)に切り捨てて返す。
// 最小側の表示に使う(「最低これくらい入る」を保守的に示す)。
// damage <= 0 または maxHP <= 0 は 0。damage > maxHP は 100% を超えた値をそのまま返す。
func DisplayPercentTenthsFloor(damage, maxHP int) int {
	if damage <= 0 || maxHP <= 0 {
		return 0
	}
	return damage * 1000 / maxHP
}

// DisplayPercentTenthsRound はダメージを HP に対する割合(0.1% 単位)に四捨五入して返す。
// 最大側の表示に使う。境界は DisplayPercentTenthsFloor と同じ。
func DisplayPercentTenthsRound(damage, maxHP int) int {
	if damage <= 0 || maxHP <= 0 {
		return 0
	}
	return (damage*2000 + maxHP) / (2 * maxHP)
}

// DisplayPercentRangeTenths はダメージ幅の表示%(0.1% 単位)を返す。
// 最小側は切り捨て、最大側は四捨五入(ADR-0010 §3.2)。どちらの丸めを使うかを
// 取り違えないよう、幅の表示はこのメソッドを正規の入口にする。
func (r DamageResult) DisplayPercentRangeTenths() (minTenths, maxTenths int) {
	return DisplayPercentTenthsFloor(r.MinDamage(), r.DefenderHP),
		DisplayPercentTenthsRound(r.MaxDamage(), r.DefenderHP)
}

// DisplayChancePercentTenths は確定数の確率を画面に出せる値(0.1% 単位)にして返す。
// 生値(ChancePercent)は確定のとき 0 なので、そのまま出すと「確定なのに 0%」になる(ADR-0010 §3.4)。
//
//	倒せない(Hits == 0) -> 0
//	確定(Guaranteed)    -> 1000(100.0%)
//	乱数                 -> 四捨五入して [1, 999]("0.0%"=不可能、"100.0%"=確定 を予約)
func (k KOChance) DisplayChancePercentTenths() int {
	switch {
	case k.Hits == 0:
		return 0
	case k.Guaranteed:
		return 1000
	}
	t := int(math.Floor(k.ChancePercent*10 + 0.5))
	return min(max(t, 1), 999)
}
