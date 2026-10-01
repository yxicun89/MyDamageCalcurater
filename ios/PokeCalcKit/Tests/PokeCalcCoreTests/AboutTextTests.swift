import XCTest

@testable import PokeCalcCore

/// P6-18(issue #328): 「このアプリについて」画面の固定文言(`AboutText`)を検証する。
/// ADR-0501「P6-18」2章・3章の確定文言をそのままピン留めする(implementer が別の文言に
/// 差し替えてしまわないように、キーワードだけでなく完全一致でも確かめる)。
///
/// spec 時点では `AboutText` はプレースホルダ(空文字列・空配列)のため、このテストは全件失敗する
/// (「この時点では失敗してよい」。CLAUDE.md spec-writer の役割どおり)。
final class AboutTextTests: XCTestCase {

    // MARK: - 2章: 非公式の注記

    private static let expectedUnofficialNotice =
        "このアプリは個人が私的に使うための非公式ツールです。"
            + "任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは関係ありません。"
            + "ポケモン・Pokémon および関連する名称は各社の商標です。"

    func testUnofficialNoticeIsNotEmpty() {
        XCTAssertFalse(AboutText.unofficialNotice.isEmpty, "非公式の注記が空")
    }

    func testUnofficialNoticeContainsKeyPhrase() {
        XCTAssertTrue(
            AboutText.unofficialNotice.contains("非公式"),
            "「非公式」という語を含んでいない: \(AboutText.unofficialNotice)"
        )
    }

    /// ADR-0501「P6-18」2章の確定文言と完全一致すること(implementer が言い回しを変えないように)。
    func testUnofficialNoticeMatchesADRWording() {
        XCTAssertEqual(AboutText.unofficialNotice, Self.expectedUnofficialNotice)
    }

    // MARK: - 3章: データの出典一覧

    /// ADR-0501「P6-18」3章の表(ADR-0002「確定した方針 / 責務の分離」と同じ順)。
    private static let expectedDataSources: [AboutText.DataSource] = [
        AboutText.DataSource(title: "ダメージ計算の検証", detail: "@smogon/calc(MIT License)"),
        AboutText.DataSource(title: "ポケモン・技・習得技の照合", detail: "Pokémon Showdown(MIT License)"),
        AboutText.DataSource(title: "日本語名・図鑑番号", detail: "PokeAPI"),
        AboutText.DataSource(title: "使用可能なポケモン等の基準", detail: "Pokémon HOME・Pokémon Champions の公式情報"),
    ]

    func testDataSourcesHasExactlyFourEntries() {
        XCTAssertEqual(AboutText.dataSources.count, 4, "3章の表は4件のはず")
    }

    func testDataSourcesMatchADRTableInOrder() {
        XCTAssertEqual(AboutText.dataSources, Self.expectedDataSources)
    }

    /// 出典名のキーワードが含まれること(完全一致テストと別に、implementer が detail の文言を
    /// 少し変えても壊れない最低条件として残す)。
    func testDataSourcesContainKnownSourceNames() {
        let expectedKeywords = ["@smogon/calc", "Pokémon Showdown", "PokeAPI", "Pokémon HOME"]
        for keyword in expectedKeywords {
            XCTAssertTrue(
                AboutText.dataSources.contains { $0.detail.contains(keyword) },
                "出典一覧に \(keyword) を含む項目が無い: \(AboutText.dataSources)"
            )
        }
    }

    /// PokeAPI・Pokémon HOME・Pokémon Champions はライセンス表記を断定しない
    /// (ADR-0002 の調査: PokeAPI はデータ自体の利用条件が README に明記されていない。
    /// 公式情報はそもそもオープンソースのライセンス概念の対象ではない)。
    func testDataSourcesDoNotInventLicensesForUnlicensedSources() {
        for source in AboutText.dataSources where source.detail.contains("PokeAPI") || source.detail.contains("Pokémon HOME") {
            XCTAssertFalse(
                source.detail.contains("License"),
                "\(source.title) にライセンスを断定して書いている(ADR-0002 は明記していない): \(source.detail)"
            )
        }
    }
}
