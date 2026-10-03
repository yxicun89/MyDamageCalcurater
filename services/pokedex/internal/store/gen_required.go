// 生成物の欠落を分かりやすくするための手書きファイル(ADR-0807)。
//
// このパッケージの db.go・models.go・pokedex.sql.go・querier.go は sqlc が
// services/pokedex/db(migrations・query)から生成し、Git に置かない。
// このファイルで「undefined: Querier」「undefined: Queries」のコンパイルエラーになったら、生成物が無い。
// リポジトリのルートで make gen を実行する(欠けているファイルの一覧は scripts/ensure-gen.sh check)。

package store

var _ Querier = (*Queries)(nil)
