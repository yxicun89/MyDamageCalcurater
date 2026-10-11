import XCTest

@testable import PokeCalcCore

/// `AppNavigationModel`(ナビゲーション案 B。ADR-0531): 下部タブ 4 つと、タブ内の区切り(ダメージの 計算/逆算/調整)の
/// 選択状態・訪れた画面を、View に依存せず持つ。
/// - タブ・区切りの並びは登録(`FeatureSpec.tab`・`order`)から決まる。区切りが無いタブは出さない
/// - タブを移っても、各タブで最後に選んだ区切りを覚えている(戻ると同じ区切り)
/// - 訪れた画面は破棄しない(View はこの集合の画面だけを作って保持する。入力を消さないため)
final class AppNavigationModelTests: XCTestCase {
    /// 登録の行の順と並び順をわざとずらす。about は右上のアイコン(タブに属さない)。
    private let specs = [
        FeatureSpec(id: "speed", order: 600, tab: .speed),
        FeatureSpec(id: "reverse", order: 200, tab: .damage),
        FeatureSpec(id: "calc", order: 100, tab: .damage),
        FeatureSpec(id: "favorites", order: 800, tab: .records),
        FeatureSpec(id: "adjust", order: 400, tab: .damage),
        FeatureSpec(id: "team", order: 300, tab: .team),
        FeatureSpec(id: "balance", order: 500),
        FeatureSpec(id: "about", order: 10_000, isToolbarEntry: true, availableWithoutServices: true),
    ]

    // MARK: 並び

    func testTabsAndSegmentsFollowRegistrationOrder() {
        let model = AppNavigationModel(specs: specs)
        XCTAssertEqual(model.tabs, [.damage, .team, .speed, .records], "タブは AppTab の宣言順")
        XCTAssertEqual(model.segments(in: .damage), ["calc", "reverse", "adjust"], "区切りは order 順")
        XCTAssertEqual(model.segments(in: .team), ["team"])
        XCTAssertEqual(model.segments(in: .speed), ["speed"])
        XCTAssertEqual(model.segments(in: .records), ["favorites"])
    }

    func testTabWithoutFeaturesIsNotShown() {
        let model = AppNavigationModel(specs: [
            FeatureSpec(id: "calc", order: 100, tab: .damage), FeatureSpec(id: "speed", order: 600, tab: .speed),
        ])
        XCTAssertEqual(model.tabs, [.damage, .speed])
        XCTAssertEqual(model.segments(in: .team), [])
        XCTAssertNil(model.selectedFeatureID(in: .team))
    }

    func testTabOfFeature() {
        let model = AppNavigationModel(specs: specs)
        XCTAssertEqual(model.tab(of: "adjust"), .damage)
        XCTAssertEqual(model.tab(of: "favorites"), .records)
        XCTAssertNil(model.tab(of: "balance"), "タブに属さない画面(構築一覧から積む)")
        XCTAssertNil(model.tab(of: "about"), "右上のアイコン")
        XCTAssertNil(model.tab(of: "unknown"))
    }

    // MARK: 初期状態

    func testInitialStateIsFirstTabFirstSegment() {
        let model = AppNavigationModel(specs: specs)
        XCTAssertEqual(model.selectedTab, .damage)
        XCTAssertEqual(model.currentFeatureID, "calc")
        XCTAssertEqual(model.visitedFeatureIDs, ["calc"], "最初に見える画面だけを作る")
        for tab in model.tabs {
            XCTAssertEqual(model.selectedFeatureID(in: tab), model.segments(in: tab).first, "\(tab) の既定は先頭の区切り")
        }
    }

    func testEmptySpecsHasNoTabs() {
        let model = AppNavigationModel(specs: [])
        XCTAssertEqual(model.tabs, [])
        XCTAssertNil(model.currentFeatureID)
        XCTAssertEqual(model.visitedFeatureIDs, [])
    }

    // MARK: タブの切り替え

    func testSelectingTabKeepsEachTabsSegment() {
        var model = AppNavigationModel(specs: specs)
        model.select(featureID: "reverse")
        model.select(tab: .team)
        XCTAssertEqual(model.selectedTab, .team)
        XCTAssertEqual(model.currentFeatureID, "team")
        model.select(tab: .damage)
        XCTAssertEqual(model.currentFeatureID, "reverse", "タブを移って戻ると、前に選んでいた区切りのまま")
    }

    func testSelectingHiddenTabIsIgnored() {
        var model = AppNavigationModel(specs: [FeatureSpec(id: "calc", order: 100, tab: .damage)])
        model.select(tab: .speed)
        XCTAssertEqual(model.selectedTab, .damage, "画面の無いタブは選べない")
        XCTAssertEqual(model.visitedFeatureIDs, ["calc"])
    }

    // MARK: 画面(区切り)の選択

    func testSelectingFeatureSwitchesTabAndSegment() {
        var model = AppNavigationModel(specs: specs)
        model.select(tab: .speed)
        model.select(featureID: "adjust")
        XCTAssertEqual(model.selectedTab, .damage, "別タブの画面を選ぶとそのタブへ移る")
        XCTAssertEqual(model.selectedFeatureID(in: .damage), "adjust")
        XCTAssertEqual(model.currentFeatureID, "adjust")
    }

    func testSelectingFeatureOutsideTabsIsIgnored() {
        var model = AppNavigationModel(specs: specs)
        model.select(featureID: "balance")
        model.select(featureID: "about")
        model.select(featureID: "unknown")
        XCTAssertEqual(model.currentFeatureID, "calc")
        XCTAssertEqual(model.visitedFeatureIDs, ["calc"])
    }

    // MARK: 訪れた画面(入力を保つ)

    func testVisitedFeaturesAccumulateAndAreNeverRemoved() {
        var model = AppNavigationModel(specs: specs)
        model.select(featureID: "reverse")
        model.select(tab: .team)
        model.select(tab: .damage)
        model.select(featureID: "calc")
        XCTAssertEqual(model.visitedFeatureIDs, ["calc", "reverse", "team"], "訪れた画面は戻っても破棄しない")
        XCTAssertFalse(model.visitedFeatureIDs.contains("adjust"), "まだ開いていない区切りは作らない")
        XCTAssertFalse(model.visitedFeatureIDs.contains("speed"))
    }

    func testVisitedFeatureIDsInTabKeepsSegmentOrder() {
        var model = AppNavigationModel(specs: specs)
        model.select(featureID: "adjust")
        model.select(featureID: "calc")
        XCTAssertEqual(model.visitedFeatureIDs(in: .damage), ["calc", "adjust"], "区切りの並び順(訪れた順ではない)")
        XCTAssertEqual(model.visitedFeatureIDs(in: .team), [])
    }

    // MARK: 文言・識別子

    func testTabTitlesAreShortAndIdentifiersStable() {
        let cases: [(tab: AppTab, title: String, identifier: String)] = [
            (.damage, "ダメージ", "tab-damage"),
            (.team, "構築", "tab-team"),
            (.speed, "素早さ", "tab-speed"),
            (.records, "記録", "tab-records"),
        ]
        for testCase in cases {
            XCTAssertEqual(testCase.tab.title, testCase.title)
            XCTAssertEqual(testCase.tab.accessibilityIdentifier, testCase.identifier)
            XCTAssertLessThanOrEqual(testCase.tab.title.count, 4, "タブのラベルは短く(\(testCase.tab))")
        }
        XCTAssertEqual(
            Set(AppTab.allCases.map(\.accessibilityIdentifier)).count, AppTab.allCases.count, "識別子は重複しない")
    }

    func testSegmentsContainerIdentifier() {
        XCTAssertEqual(AppTab.damage.segmentsAccessibilityIdentifier, "tabSegments-damage")
    }
}
