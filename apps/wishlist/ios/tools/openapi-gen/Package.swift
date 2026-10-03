// swift-tools-version: 6.4
// swift-openapi-generator を版固定で実行するための生成専用パッケージ(ダメ計 ios/tools/openapi-gen と同じ版)。
// アプリやパッケージの依存には入らない。`ios/scripts/openapi-gen.sh` から
// `swift build -c release --product swift-openapi-generator` した実行ファイルを直接呼ぶ。
import PackageDescription

let package = Package(
    name: "openapi-gen",
    platforms: [.macOS(.v27)],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-generator", exact: "1.13.1"),
    ],
    swiftLanguageModes: [.v6]
)
