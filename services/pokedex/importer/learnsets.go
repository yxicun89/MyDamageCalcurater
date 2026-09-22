package importer

// 習得技の解決(基本種の継承。ADR-0101 §5)。ADR-0103 §7: 進化前(prevo)をたどるかどうかは
// レギュレーションの `inheritFromPrevo` によるデータ駆動(Go に mod 名をハードコードしない)。
//
// Showdown 本体のソース(dex-species.js の learnsetParent)を読んだところ、champions mod は
// 進化前をたどる継承を世代を問わず常に無効化していると判明した(§10 の実データ確認: 生成した
// resolved(s) を PS の TeamValidator.checkCanLearn と標本突き合わせしたところ、進化前からの継承を
// 含む技はことごとく「学習不可」と判定された)。そのため M-C は inheritFromPrevo=false(ADR-0101 §5
// の元の規則: 自分の学習元、無ければ基本種の学習元を1段だけ)。true のときは、Showdown の
// TeamValidator が学習元の世代も見る(minSourceGen 未満の学習元は認めない)ことに合わせて、
// 進化前を何段でもたどりつつ世代でも絞る(将来 inheritFromPrevo=true を使うレギュレーションのために
// 実装を残す)。

import "fmt"

// learnsetResolver は Showdown の種族 ID から「resolved(s)」を計算する。
// rules.InheritFromPrevo が false(既定・M-C): resolved(s) = 自分の学習元(minSourceGen 以上の
// 世代のものだけ)。無ければ基本種(フォーム・メガの base species)の学習元を1段だけ。
// rules.InheritFromPrevo が true: 上に加えて、進化前(prevo)の resolved を何段でも足す。
type learnsetResolver struct {
	byID      map[string]ShowdownSpecies
	byName    map[string]ShowdownSpecies
	learnsets map[string]map[string]int // species id -> move id -> 学習元の最大世代
	rules     learnsetRules

	resolved   map[string]map[string]bool // resolved(s)(memo)
	resolvedNo map[string]map[string]bool // 進化前を含まない解決結果(Inherited の算出用)
}

func newLearnsetResolver(species []ShowdownSpecies, learnsets map[string]map[string]int, rules learnsetRules) *learnsetResolver {
	byID := make(map[string]ShowdownSpecies, len(species))
	byName := make(map[string]ShowdownSpecies, len(species))
	for _, s := range species {
		byID[s.ID] = s
		byName[s.Name] = s
	}
	return &learnsetResolver{
		byID: byID, byName: byName, learnsets: learnsets, rules: rules,
		resolved:   map[string]map[string]bool{},
		resolvedNo: map[string]map[string]bool{},
	}
}

// ownMoves は種族 id 自身の学習元のうち、rules.MinSourceGen 以上の世代のものだけの集合。
func (r *learnsetResolver) ownMoves(id string) map[string]bool {
	set := map[string]bool{}
	for m, gen := range r.learnsets[id] {
		if gen >= r.rules.MinSourceGen {
			set[m] = true
		}
	}
	return set
}

// resolve は resolved(s) を返す(rules.InheritFromPrevo のときだけ進化前を含む)。
func (r *learnsetResolver) resolve(id string) (map[string]bool, error) {
	return r.resolveStack(id, map[string]bool{})
}

func (r *learnsetResolver) resolveStack(id string, stack map[string]bool) (map[string]bool, error) {
	if set, ok := r.resolved[id]; ok {
		return set, nil
	}
	if stack[id] {
		return nil, fmt.Errorf("%w: 種族 %q の進化前(prevo)の指定が循環している", ErrInvalidData, id)
	}
	stack[id] = true
	defer delete(stack, id)

	sp := r.byID[id]
	var set map[string]bool
	if _, ok := r.learnsets[id]; ok {
		set = r.ownMoves(id)
		if r.rules.InheritFromPrevo && sp.Prevo != "" {
			prevoSp, ok := r.byName[sp.Prevo]
			if !ok {
				return nil, fmt.Errorf("%w: 種族 %q の進化前(prevo)%q が Showdown に無い", ErrInvalidData, id, sp.Prevo)
			}
			prevoSet, err := r.resolveStack(prevoSp.ID, stack)
			if err != nil {
				return nil, err
			}
			for m := range prevoSet {
				set[m] = true
			}
		}
	} else if baseID := toID(sp.BaseSpecies); baseID != id {
		baseSet, err := r.resolveStack(baseID, stack)
		if err != nil {
			return nil, err
		}
		set = map[string]bool{}
		for m := range baseSet {
			set[m] = true
		}
	} else {
		set = map[string]bool{}
	}
	r.resolved[id] = set
	return set, nil
}

// resolveWithoutPrevo は進化前を継がない解決結果(自分の学習元、無ければ基本種の学習元を1段だけ。
// どちらも minSourceGen 以上の世代のものだけ)。rules.InheritFromPrevo に関わらずこの規則。
// Learnsets.Inherited の算出に使う(InheritFromPrevo が false なら resolve と常に同じ集合になり、
// Inherited は常に 0 になる)。
func (r *learnsetResolver) resolveWithoutPrevo(id string) map[string]bool {
	if set, ok := r.resolvedNo[id]; ok {
		return set
	}
	sp := r.byID[id]
	var set map[string]bool
	if _, ok := r.learnsets[id]; ok {
		set = r.ownMoves(id)
	} else if _, ok := r.learnsets[toID(sp.BaseSpecies)]; ok {
		set = r.ownMoves(toID(sp.BaseSpecies))
	} else {
		set = map[string]bool{}
	}
	r.resolvedNo[id] = set
	return set
}
