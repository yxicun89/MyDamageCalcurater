import Foundation
import XCTest

// AC-IOS-5-3j: ビルド設定・Share Extension の JavaScript の取り決め（ファイルの中身を文字列で見る）。
// docs/phase5-spec.md 5-3「決めたこと」。ユーザー決定 2026-10-11。ios/ の実ファイルを #filePath からたどって読む。
final class BuildConfigTests: XCTestCase {
    /// ios/（WishlistKit の 1 つ上）
    private let iosDir: URL = URL(fileURLWithPath: #filePath)
        .deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()

    private func read(_ relative: String) throws -> String {
        let url = iosDir.appendingPathComponent(relative)
        return try String(contentsOf: url, encoding: .utf8)
    }

    /// AC-IOS-5-3j-1: 共有拡張の JavaScript がある。`ExtensionPreprocessingJS` に run / finalize があり、og:image・og:title を集める
    func testExtensionPreprocessingScript() throws {
        let js = try read("WishlistShare/ExtensionPreprocessing.js")
        XCTAssertTrue(js.contains("ExtensionPreprocessingJS"))
        XCTAssertTrue(js.contains("run:") || js.contains("run ="), "run がある")
        XCTAssertTrue(js.contains("finalize:") || js.contains("finalize ="), "finalize がある")
        XCTAssertTrue(js.contains("completionFunction"), "run が値を Swift へ渡す")
        XCTAssertTrue(js.contains("og:image"))
        XCTAssertTrue(js.contains("og:title"))
        XCTAssertTrue(js.contains("document.title"), "og:title が無ければ document.title")
        XCTAssertTrue(js.contains("img"), "og:image が無ければ最も大きい <img>")
        for key in ["url", "title", "imageURL"] { XCTAssertTrue(js.contains(key), "結果のキー \(key)") }
    }

    /// AC-IOS-5-3j-2: 拡張の Info.plist が JavaScript を前処理に指定し、接続先のキーを持つ。トークンの語は無い
    func testShareExtensionInfoPlist() throws {
        let plist = try read("WishlistShare-Info.plist")
        XCTAssertTrue(plist.contains("NSExtensionJavaScriptPreprocessingFile"))
        XCTAssertTrue(plist.contains("<string>ExtensionPreprocessing</string>"))
        XCTAssertTrue(plist.contains("<key>WishlistAPIBaseURL</key>"))
        XCTAssertTrue(plist.contains("$(WISHLIST_API_BASE_URL)"))
        XCTAssertFalse(plist.lowercased().contains("token"))
    }

    /// AC-IOS-5-3j-3: 本体の Info.plist も同じキーで接続先を持つ
    func testAppInfoPlist() throws {
        let plist = try read("Wishlist-Info.plist")
        XCTAssertTrue(plist.contains("<key>WishlistAPIBaseURL</key>"))
        XCTAssertTrue(plist.contains("$(WISHLIST_API_BASE_URL)"))
        XCTAssertFalse(plist.lowercased().contains("token"))
    }

    /// AC-IOS-5-3j-4: Local.xcconfig は git に入れず、見本（.example）をコミットする。見本に実在のホストを書かない
    func testLocalXcconfigIsIgnoredAndHasAnExample() throws {
        let gitignore = try read(".gitignore")
        XCTAssertTrue(gitignore.split(separator: "\n").contains { $0.trimmingCharacters(in: .whitespaces).hasSuffix("Local.xcconfig") })
        let example = try read("Config/Local.xcconfig.example")
        XCTAssertTrue(example.contains("WISHLIST_API_BASE_URL"))
        XCTAssertTrue(example.contains("$()"), "xcconfig では `//` がコメントになるので https:/$()/host の形で書く")
    }

    /// AC-IOS-5-3j-5: アプリのソースにトークン・Bearer・接続設定の保存が残っていない
    func testAppSourcesHaveNoTokenOrConnectionSettings() throws {
        let fm = FileManager.default
        let dirs = ["Wishlist", "WishlistShare", "WishlistKit/Sources/WishlistCore"]
        var offenders: [String] = []
        for dir in dirs {
            let root = iosDir.appendingPathComponent(dir)
            for case let url as URL in fm.enumerator(at: root, includingPropertiesForKeys: nil) ?? NSEnumerator()
            where url.pathExtension == "swift" {
                let text = try String(contentsOf: url, encoding: .utf8)
                for needle in ["Bearer", "AuthPolicy", "ConnectionSettingsViewModel", "WishlistSettingsStore", "UserDefaultsSettingsStore"]
                where text.contains(needle) { offenders.append("\(url.lastPathComponent): \(needle)") }
            }
        }
        XCTAssertEqual(offenders, [])
    }
}
