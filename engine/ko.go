package engine

// 確定数/乱数n発の算出。
//
// 16段階のロールは各 1/16 の確率で等確率に出る。n 発の合計ダメージが防御側 HP 以上に
// なれば「倒せる」。倒すのに必要な最小の n を求め、最小ロールでも n 発で倒せるなら
// 「確定n発」、そうでなければ「乱数n発」+ n 発で倒せる確率(%)を返す。

// KOChance は確定数/乱数n発の結果。
type KOChance struct {
	Hits          int     // 倒すのに必要な最小攻撃回数(0 = 倒せない)
	Guaranteed    bool    // 最小ロールでも Hits 回で確実に倒せる(確定n発)
	ChancePercent float64 // Guaranteed=false のとき、Hits 回で倒せる確率(%)
}

// computeKO は結果から確定数を求める。多段技は1回の使用 = 回数ぶんの独立な16段階として数える(ADR-0142 §3)。
// 単発の技(HitRolls が無い)は ComputeKO と同じ。
func (r DamageResult) computeKO() KOChance {
	return computeKOHits(r.Rolls, r.HitRolls, r.DefenderHP)
}

// ComputeKO は16段階ロールと HP から確定数/乱数n発を求める(単発用)。
func ComputeKO(rolls [16]int, hp int) KOChance {
	return computeKOHits(rolls, nil, hp)
}

// computeKOHits は rolls(1回の使用の合計の16段階)と、多段技の1発ごとの16段階 hitRolls(単発は nil)から
// 確定数を求める。n 回の使用の確率は n × 回数 個の独立な乱数の合計で数える。
func computeKOHits(rolls [16]int, hitRolls [][16]int, hp int) KOChance {
	maxDmg := rolls[15]
	minDmg := rolls[0]
	if maxDmg <= 0 || hp <= 0 {
		return KOChance{Hits: 0} // 倒せない(無効・ダメージ0)
	}
	// 最大ロールで倒せる最小の攻撃回数。
	n := (hp + maxDmg - 1) / maxDmg
	// 最小ロールでも n 発で倒せるなら確定。
	if minDmg*n >= hp {
		return KOChance{Hits: n, Guaranteed: true}
	}
	return KOChance{
		Hits:          n,
		Guaranteed:    false,
		ChancePercent: koProbabilityHits(rolls, hitRolls, hp, n) * 100,
	}
}

// koProbabilityHits は n 回の使用で hp 以上になる確率。単発(hitRolls が 2 未満)は koProbability と同じ。
func koProbabilityHits(rolls [16]int, hitRolls [][16]int, hp, n int) float64 {
	if len(hitRolls) < 2 {
		return koProbability(rolls, hp, n)
	}
	return koProbabilitySeq(hitRolls, hp, n)
}

// koProbability は n 発の合計が hp 以上になる確率を、16ロールの畳み込みで厳密に求める。
// hp 以上の合計は1つのバケット(index=hp)に吸収する。
func koProbability(rolls [16]int, hp, n int) float64 {
	return koProbabilitySeq([][16]int{rolls}, hp, n)
}

// koProbabilitySeq は seq(1回の使用の各発の16段階)を n 回繰り返した全発の合計が hp 以上になる確率。
func koProbabilitySeq(seq [][16]int, hp, n int) float64 {
	// dist[s] = ちょうど s ダメージ(s<hp)の確率、dist[hp] = hp 以上(=KO)の確率。
	dist := make([]float64, hp+1)
	dist[0] = 1
	const p = 1.0 / 16.0
	next := make([]float64, hp+1)
	for draw := 0; draw < n*len(seq); draw++ {
		rolls := seq[draw%len(seq)]
		clear(next)
		for s := 0; s <= hp; s++ {
			if dist[s] == 0 {
				continue
			}
			if s == hp {
				next[hp] += dist[s] // 既に KO は吸収し続ける
				continue
			}
			for _, r := range rolls {
				ns := s + r
				if ns >= hp {
					ns = hp
				}
				next[ns] += dist[s] * p
			}
		}
		dist, next = next, dist
	}
	return dist[hp]
}
