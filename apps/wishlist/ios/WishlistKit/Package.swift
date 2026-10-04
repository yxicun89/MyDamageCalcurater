// swift-tools-version: 6.4
// WishlistKit: wishlist iOS アプリの View 以外(API 生成物・ドメイン・サービス・キャッシュ・ViewModel)を置くパッケージ。
// ダメ計の ios/PokeCalcKit と同じ構成。macOS の `swift test` と、シミュレータの
// `xcodebuild test -scheme WishlistKit-Package` の両方で同じテストが走る(docs/phase2-ios-spec.md)。
// 依存の版は PokeCalcKit と同じで完全固定(coding-rules §1)。いずれも Apache-2.0。
import PackageDescription

let package = Package(
    name: "WishlistKit",
    platforms: [.iOS(.v27), .macOS(.v27)],
    products: [
        .library(name: "WishlistAPI", targets: ["WishlistAPI"]),
        .library(name: "WishlistCore", targets: ["WishlistCore"]),
    ],
    dependencies: [
        .package(url: "https://github.com/apple/swift-openapi-runtime", exact: "1.12.1"),
        .package(url: "https://github.com/apple/swift-openapi-urlsession", exact: "1.3.1"),
        // 生成コード(Generated/*.swift)が `import HTTPTypes` するので明示する。
        .package(url: "https://github.com/apple/swift-http-types", exact: "1.8.0"),
    ],
    targets: [
        // swift-openapi-generator の生成物だけ(手で編集しない。`make wishlist-ios-gen` で作る)
        .target(
            name: "WishlistAPI",
            dependencies: [
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ]
        ),
        // ドメインの型・検索ワードとディープリンク・WishlistService(API 実装と fake)・キャッシュ・設定・ViewModel
        .target(
            name: "WishlistCore",
            dependencies: [
                "WishlistAPI",
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "OpenAPIURLSession", package: "swift-openapi-urlsession"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ]
        ),
        .testTarget(
            name: "WishlistCoreTests",
            dependencies: [
                "WishlistCore",
                .product(name: "OpenAPIRuntime", package: "swift-openapi-runtime"),
                .product(name: "HTTPTypes", package: "swift-http-types"),
            ],
            // testdata/query-cases.json のコピー(scripts/sync-testdata.sh で同期。シンボリックリンクにしない)
            resources: [.copy("Resources/query-cases.json")]
        ),
    ],
    // 言語モードを明示する(既定に任せない)。
    swiftLanguageModes: [.v6]
)
