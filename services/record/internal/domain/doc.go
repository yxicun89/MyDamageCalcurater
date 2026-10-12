// Package domain は record-svc の純粋な核(ADR-0811・ADR-0254)。
//
// お気に入りの内容(個体・計算の入力)の検証と正規化(既定値を補い、キーの順序が決まった JSON にする)、
// その上限(SP・レベル・ランク・ラベルの長さ・内容の大きさ。G6)と、失敗を表す語彙(Code・Error)を持つ。
// SP・レベルの上限は engine の定数(calc-svc が使う正)を参照する。
// HTTP・DB・NATS・時刻の取得を持たない。import してよいのは engine と標準ライブラリだけ。
// どの値が契約の列挙に含まれるかは契約(api/openapi.yaml)が決めるので、その判定は adapters が Enums で渡す。
package domain
