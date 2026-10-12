// Package domain は team-svc の純粋な核(ADR-0811・ADR-0255)。
//
// 構築(Team・Member・StatBlock)の形と、「マスタを引かなくても判断できる」入力の検証(構築名・メンバー数・
// 技の数と重複・ニックネームと ID の長さ・SP の範囲と合計。ADR-0213 §3)、失敗を表す語彙(Code・Error)を持つ。
// HTTP・DB・NATS・時刻の取得を持たない。import してよいのは標準ライブラリだけ。
// team-svc はマスタ(pokedex)を引かない(CLAUDE.md 絶対ルール4)ので、ID の実在は見ない。
package domain
