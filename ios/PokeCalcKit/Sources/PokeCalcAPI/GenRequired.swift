// 生成物の欠落を分かりやすくするための手書きファイル(ADR-0806)。
//
// このターゲットの Generated/ は api/openapi.yaml から swift-openapi-generator で作り、Git に置かない。
// このファイルで「cannot find type 'Client' in scope」になったら、生成物が無い。
// リポジトリのルートで make ios-gen を実行してから、もう一度ビルドする(Xcode で直接開く前も同じ)。
// SwiftPM はソースの無いターゲットを作れないので、生成物が無くてもこのファイルがあれば上の案内に辿り着ける。

private typealias GeneratedClientRequired = Client
