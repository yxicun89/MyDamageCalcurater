// 生成物の欠落を分かりやすくするための手書きファイル(ADR-0807)。
//
// openapi.gen.go は services/speed/api/openapi.yaml から生成し、Git に置かない。
// このファイルで「undefined: RegisterHandlers」のコンパイルエラーになったら、生成物が無い。
// リポジトリのルートで make gen を実行する(欠けているファイルの一覧は scripts/ensure-gen.sh check)。

package api

var _ = RegisterHandlers
