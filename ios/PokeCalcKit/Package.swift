// swift-tools-version: 6.4
// PokeCalcKit: iOS アプリの View 以外(API 生成物・ドメイン・モック・デザイントークン)を置くパッケージ(ADR-0017 §1)。
// macOS の `swift test` と iOS シミュレータの `xcodebuild test -scheme PokeCalcKit-Package` の両方で同じテストが走る。
// 依存の版は完全固定(coding-rules §1)。ライセンスは いずれも Apache-2.0(apple/swift-openapi-* と apple/swift-http-types)。
// ツール版・言語モードは導入できる最新(Swift 6.4 / Xcode 27。ユーザー決定)に揃える。
import PackageDescription

let package = Package(
    name: "PokeCalcKit",
    // iOS 27 未満は対象外(ユーザー決定。ADR-0017 §1 は別途更新される)。
    platforms: [.iOS(.v27), .macOS(.v27)],
    products: [
        .library(name: "PokeCalcAPI", targets: ["PokeCalcAPI"]),
        .library(name: "PokeCalcCore", targets: ["PokeCalcCore"]),
        .library(name: "PokeCalcDesign", targets: ["PokeCalcDesign"]),
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-runtime", exact: "1.12.1"),
        .package(url: "https://github.com/apple/swift-openapi-urlsession", exact: "1.3.1"),
        // 生成コード(Generated/*.swift)が `import HTTPTypes` するので明示する。版は openapi-runtime 1.12.1 の解決結果に合わせて固定。
        .package(url: "https://github.com/apple/swift-http-types", exact: "1.8.0"),
    ],
    targets: [
        // swift-openapi-generator の生成物だけ(手で編集しない。`make ios-gen` で作る。ADR-0017 §2)
        .target(
            name: "PokeCalcAPI",
            dependencies: [
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ]
        ),
        // デザイントークン(docs/design.md と同じ名前・値)。SwiftUI だけに依存する
        .target(name: "PokeCalcDesign"),
        // ドメインの型・PokeCalcService・API 実装・モック(架空データは Resources/)
        .target(
            name: "PokeCalcCore",
            dependencies: [
                "PokeCalcAPI",
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "OpenAPIURLSession", package: "swift-openapi-urlsession"),
            ],
            resources: [.process("Resources")]
        ),
        .testTarget(name: "PokeCalcDesignTests", dependencies: ["PokeCalcDesign"]),
        .testTarget(
            name: "PokeCalcCoreTests",
            dependencies: [
                "PokeCalcCore",
                "PokeCalcAPI",
                // PokeType の全ケースにタイプ色があることの同期テストに使う
                "PokeCalcDesign",
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ]
        ),
    ],
    // 言語モードを明示する(既定に任せない。ユーザー決定で Swift 6 に揃える)。
    swiftLanguageModes: [.v6]
)
