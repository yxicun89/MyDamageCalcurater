import XCTest

/// 画面への移り方の 1 か所の集約(ナビゲーション案 B。ADR-0531)。各テストは画面の入口 ID(旧ホームのピルの ID)で
/// `app.openScreen("openCalcScreen")` と呼ぶ。下部タブ・タブ内の区切りの組み立てが変わったら、ここだけを直す。
///
/// - 計算・逆算・調整(判定を出すときは判定も)は「ダメージ」タブの上部の区切り。区切りのボタンは旧 ID をそのまま持つ
/// - 構築・素早さ・お気に入り/履歴はそれぞれのタブ(タブのボタンの識別子は `tab-<タブ>`)
/// - タイプバランスは構築タブの一覧の行(`openBalanceScreen`)
/// - 「このアプリについて」は今のタブの右上のアイコン(`openAboutScreen`)
enum UITestNavigation {
    static let existenceTimeout: TimeInterval = 5

    /// 下部タブのボタンの識別子(`AppTab.accessibilityIdentifier`)。
    enum TabID {
        static let damage = "tab-damage"
        static let team = "tab-team"
        static let speed = "tab-speed"
        static let records = "tab-records"
    }

    /// タブのラベル(`AppTab.title`)。識別子で見つからないときの引き直しに使う。
    static let tabLabels: [String: String] = [
        TabID.damage: "ダメージ", TabID.team: "構築", TabID.speed: "素早さ", TabID.records: "記録",
    ]

    /// 入口 ID → その画面が入っているタブ。区切りの画面は、タブを開いたあと区切りのボタン(入口 ID)を押す。
    static let tabForEntry: [String: String] = [
        "openCalcScreen": TabID.damage,
        "openReverseScreen": TabID.damage,
        "openAdjustScreen": TabID.damage,
        "openJudgeScreen": TabID.damage,
        "openTeamListScreen": TabID.team,
        "openBalanceScreen": TabID.team,
        "openSpeedScreen": TabID.speed,
        "openFavoritesScreen": TabID.records,
    ]

    /// タブを開いたあと、さらに入口 ID のボタンを押す画面(タブ内の区切り・一覧の行)。
    static let entriesInsideTab: Set<String> = [
        "openCalcScreen", "openReverseScreen", "openAdjustScreen", "openJudgeScreen", "openBalanceScreen",
    ]
}

@MainActor
extension XCUIApplication {
    /// 下部タブのボタン(識別子で引く。識別子が引けることの検査は `AppNavigationUITests` が持つ)。
    func tabButton(_ tabID: String) -> XCUIElement {
        tabBars.buttons[tabID]
    }

    /// 下部タブを選ぶ(既に選ばれていても押さない: 選択中のタブを押し直すとスタックが根へ戻るため)。
    /// 識別子で見つからないときはラベルで引き直す(移り方の補助なので、識別子の伝わり方の揺れで各画面のテストを落とさない)。
    /// そのときは画面の要素の一覧を添付して残す。
    func selectTab(_ tabID: String, file: StaticString = #filePath, line: UInt = #line) {
        var tab = tabButton(tabID)
        if !tab.waitForExistence(timeout: UITestNavigation.existenceTimeout),
            let label = UITestNavigation.tabLabels[tabID]
        {
            let attachment = XCTAttachment(string: debugDescription)
            attachment.name = "タブ \(tabID) が識別子で見つからなかったときの要素の一覧"
            attachment.lifetime = .keepAlways
            XCTContext.runActivity(named: "タブ \(tabID) をラベルで引き直す") { $0.add(attachment) }
            print("UITestNavigation: タブ \(tabID) が識別子で見つからない。タブバー: \(tabBars.debugDescription)")
            tab = tabBars.buttons[label]
        }
        XCTAssertTrue(
            tab.waitForExistence(timeout: UITestNavigation.existenceTimeout), "タブ \(tabID) が見つからない",
            file: file, line: line)
        if !tab.isSelected { tab.tap() }
    }

    /// 入口 ID の画面を開く(タブ → 区切り/行。右上のアイコンはそのまま押す)。
    func openScreen(_ entryID: String, file: StaticString = #filePath, line: UInt = #line) {
        if let tabID = UITestNavigation.tabForEntry[entryID] {
            selectTab(tabID, file: file, line: line)
            guard UITestNavigation.entriesInsideTab.contains(entryID) else { return }
        }
        let entry = descendants(matching: .any)[entryID]
        XCTAssertTrue(
            entry.waitForExistence(timeout: UITestNavigation.existenceTimeout), "入口 \(entryID) が見つからない",
            file: file, line: line)
        if !entry.isHittable { swipeUpUntilHittable(entry) }
        entry.tap()
    }

    /// 入口が画面外(大きい文字で一覧の下など)にあるとき、押せる位置まで送る。
    private func swipeUpUntilHittable(_ element: XCUIElement, maxSwipes: Int = 6) {
        var swipes = 0
        while !element.isHittable && swipes < maxSwipes {
            swipeUp()
            swipes += 1
        }
    }
}
