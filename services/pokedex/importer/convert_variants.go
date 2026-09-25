package importer

// Showdown の Dex は、同じ技のタイプ違いの版(例: 型ごとの別名を持つ技)を、名前は別のまま
// 同じ id で返す(id は基本形の名前から作られる)。そのまま ID で引く map に入れると後の行が黙って
// 勝ち、重複検査(#311)を入れた後は取り込み全体が止まる。名前から作った ID が id と一致する行を
// 正規の1件として残し、それ以外の版は警告に出して除く(ADR-0115 追記)。

// foldShowdownMoveVariants は同じ id を持つ Showdown の技を正規の1件にまとめる。まとめられるのは、
// 同じ id の行のうち toID(名前) == id の行がちょうど1件あるときだけ。それ以外(完全な重複や、
// 正規の行が無い・複数ある)はそのまま返し、checkUniqueSourceIDs に止めさせる。
func foldShowdownMoveVariants(moves []ShowdownMove) ([]ShowdownMove, []Finding) {
	byID := make(map[string][]int, len(moves))
	for i, m := range moves {
		byID[m.ID] = append(byID[m.ID], i)
	}
	drop := make(map[int]bool)
	var warnings []Finding
	for id, idx := range byID {
		if len(idx) < 2 {
			continue
		}
		canonical := -1
		count := 0
		for _, i := range idx {
			if toID(moves[i].Name) == id {
				canonical = i
				count++
			}
		}
		if count != 1 {
			continue
		}
		for _, i := range idx {
			if i == canonical {
				continue
			}
			drop[i] = true
			warnings = append(warnings, Finding{Kind: KindMoveVariantFolded, ID: toID(moves[i].Name), Detail: id})
		}
	}
	if len(drop) == 0 {
		return moves, nil
	}
	out := make([]ShowdownMove, 0, len(moves)-len(drop))
	for i, m := range moves {
		if !drop[i] {
			out = append(out, m)
		}
	}
	sortFindings(warnings)
	return out, warnings
}
