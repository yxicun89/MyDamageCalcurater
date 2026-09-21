// swift-tools-version: 6.4
// swift-openapi-generator を版固定で実行するための生成専用パッケージ(ADR-0017 §2)。
// アプリやパッケージの依存には入らない。`ios/scripts/openapi-gen.sh` から
// `swift build -c release --product swift-openapi-generator` した実行ファイルを直接呼ぶ
// (`swift run` は使わない。`swift build` の方が生成器自身の標準出力・終了コードを汚さない)。
// ツール版・言語モードは導入できる最新(Swift 6.4 / Xcode 27。ユーザー決定)に揃える。
import PackageDescription

let package = Package(
    name: "openapi-gen",
    platforms: [.macOS(.v27)],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-generator", exact: "1.13.1"),
    ],
    swiftLanguageModes: [.v6]
)
