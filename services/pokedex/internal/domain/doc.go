// Package domain は pokedex-svc の純粋な核(ADR-0811・ADR-0180)。
//
// マスタの中身そのものの検証は共通マスタ(services/internal/master)と engine が持つ。ここに置くのは、
// pokedex-svc が DB を呼ぶ前に行う入力の検証(limit・offset・format・種族キー・ids の件数)、検索の LIKE の
// パターンの組み立て、効果定義の JSON を数値の字面を保って読むこと、dataVersion の文字列、read model の
// 特性の効果の正規化と、失敗を表す語彙(Code・Error)。
// HTTP・DB・NATS・時刻の取得を持たない。import してよいのは engine・services/internal/master・標準ライブラリだけ。
package domain
