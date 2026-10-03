// 生成物の欠落を分かりやすくするための手書きファイル(ADR-0171)。
//
// openapi.gen.go は api/openapi.yaml から生成し、Git に置かない。
// このファイルで「undefined: GetSwagger」のコンパイルエラーになったら、生成物が無い。
// リポジトリのルートで make gen を実行する(欠けているファイルの一覧は scripts/ensure-gen.sh check)。

package api

var _ = GetSwagger
