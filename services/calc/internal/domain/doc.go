// Package domain は calc-svc の純粋な核(ADR-0811・ADR-0252)。
//
// 計算そのものの核は engine で、ここに置くのは calc-svc が engine を呼ぶ前に行う入力の検証だけ
// (列挙の検証・個体の検証・メガシンカ後の持ち物の規則・観測・対戦の状態・調整の値域)と、失敗を表す語彙(Code・Error)。
// HTTP・DB・NATS・時刻の取得を持たない。import してよいのは engine と標準ライブラリだけ。
package domain
