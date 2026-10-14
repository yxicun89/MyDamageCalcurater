import XCTest

@testable import PokeCalcCore

/// G-05 第 5 弾(ADR-0541。ADR-0501「G-05 計算の作り直し」章)。計算画面に出す文字を、
/// 「名前 / 短いラベル / 数値・結果 / 状態の一言」の 4 種に絞る。文を置いてよいのは、エラー・空の状態・取り消せない操作の確認・
/// 単位が分からない欄の 1 行だけ(docs/design.md「文を減らす決まり」)。語は Web の `web/src/i18n/ja.ts`(`calcScreenText`・`attackerStatText`)と同じ。
final class G05CalcTextTests: XCTestCase {

    // MARK: - 見出し・状態の一言(Web の attackerRegionLabel・defenderRegionLabel・loadingNotice・resultsListLabel)

    func testSideHeadingsFollowWeb() {
        XCTAssertEqual(CalcScreenText.attackerHeading, "攻撃側")
        XCTAssertEqual(CalcScreenText.defenderHeading, "防御側")
    }

    func testLoadingIsAStaticShortWord() {
        XCTAssertEqual(CalcScreenText.loading, "計算中")
        XCTAssertFalse(CalcScreenText.loading.contains("…"), "回って見える「…」を付けない")
    }

    func testResultsListLabelFollowsWeb() {
        XCTAssertEqual(CalcScreenText.resultsListLabel, "計算結果")
    }

    func testDisclosureValuesMatchSpeedScreenWords() {
        XCTAssertEqual(CalcScreenText.disclosureOpened, "開いています")
        XCTAssertEqual(CalcScreenText.disclosureClosed, "閉じています")
    }

    // MARK: - SP の − / + の名前は用語集の語(「<欄の名前>を増やす / 減らす」)

    func testSPButtonNamesFollowGlossary() {
        XCTAssertEqual(AttackerStatLabels.spIncrease(.atk), "攻撃のSPを増やす")
        XCTAssertEqual(AttackerStatLabels.spDecrease(.atk), "攻撃のSPを減らす")
        XCTAssertEqual(AttackerStatLabels.spIncrease(.spa), "特攻のSPを増やす")
        XCTAssertEqual(AttackerStatLabels.spDecrease(.spa), "特攻のSPを減らす")
        for stat in AttackStat.allCases {
            XCTAssertTrue(AttackerStatLabels.spIncrease(stat).hasPrefix(AttackerStatLabels.spLabel(stat)), "欄の名前を含む(WCAG 2.5.3)")
            XCTAssertTrue(AttackerStatLabels.spDecrease(stat).hasPrefix(AttackerStatLabels.spLabel(stat)))
        }
    }

    // MARK: - 変えない語(読み上げ名・Web と同じ語。既存テストと同じ値をここでも固定する)

    func testExistingWordsStay() {
        XCTAssertEqual(AttackerStatLabels.spLabel(.atk), "攻撃のSP")
        XCTAssertEqual(AttackerStatLabels.natureGroupLabel(.spa), "特攻の性格補正")
        XCTAssertEqual(AttackerStatLabels.modifierName(.up), "上昇")
        XCTAssertEqual(AttackerStatLabels.modifierName(.neutral), "補正なし")
        XCTAssertEqual(AttackerStatLabels.modifierName(.down), "下降")
        XCTAssertEqual(AttackerStatLabels.usedSuffix, "(この技で使用)")
        XCTAssertEqual(AttackerStatLabels.custom, "カスタム")
        XCTAssertEqual(AttackerStatLabels.keyboardDone, "完了")
        XCTAssertEqual(AttackerStatLabels.spInvalid(.atk), "攻撃のSPは0〜32の整数で入力してください")
        XCTAssertEqual(CalcConditionLabels.sectionTitle, "詳細")
        XCTAssertEqual(CalcConditionLabels.critical, "急所")
        XCTAssertEqual(CalcConditionLabels.burn, "やけど")
        XCTAssertEqual(EffectivenessLabel.text(1), "等倍")
        XCTAssertEqual(EffectivenessLabel.text(2), "ばつぐん(×2)")
        XCTAssertEqual(EffectivenessLabel.text(0.5), "いまひとつ(×0.5)")
        XCTAssertEqual(EffectivenessLabel.text(0), "効果なし")
    }

    /// 連続技の節の語は変えない(ADR-0535。この弾は部品の置き換えだけ)。
    func testComboWordsStay() {
        XCTAssertEqual(ComboText.sectionLabel, "連続技")
        XCTAssertEqual(ComboText.addStepButton, "次の技")
        XCTAssertEqual(ComboText.closeButton, "閉じる")
        XCTAssertEqual(ComboText.loading, "連続技を計算中…")
        XCTAssertEqual(ComboText.retryButton, "もう一度")
        XCTAssertEqual(ComboText.note, "場は全手で同じ状態が続きます。命中は考えません。")
        XCTAssertEqual(ComboText.speedFieldToggle, "素早さの場")
    }

    // MARK: - 常に画面に出る文字に「。」を置かない(文を置いてよいのは確認・エラー・単位の 1 行だけ)

    func testAlwaysVisibleStringsAreNotSentences() {
        let visible: [(String, String)] = [
            ("attackerHeading", CalcScreenText.attackerHeading),
            ("defenderHeading", CalcScreenText.defenderHeading),
            ("loading", CalcScreenText.loading),
            ("resultsListLabel", CalcScreenText.resultsListLabel),
            ("spIncrease", AttackerStatLabels.spIncrease(.atk)),
            ("spDecrease", AttackerStatLabels.spDecrease(.atk)),
            ("sectionTitle", CalcConditionLabels.sectionTitle),
            ("custom", AttackerStatLabels.custom),
            ("usedSuffix", AttackerStatLabels.usedSuffix),
            ("effectiveness", EffectivenessLabel.text(2)),
        ]
        for (name, text) in visible {
            XCTAssertFalse(text.isEmpty, "\(name) が空")
            XCTAssertFalse(text.contains("。"), "\(name) は一言(文にしない): \(text)")
        }
    }

    // MARK: - 禁止語(docs/glossary.md「言い換える語」。`GlossaryTests` と同じ一覧を持つ。ここだけで完結させる)

    func testNewStringsHaveNoForbiddenWords() {
        let forbidden = [
            "観測", "推定結果", "近い候補", "プリセット", "指数", "16n", "最小の振り方", "API", "WASM",
            "実行場所", "計算モード", "マスタ", "識別子", "識別情報", "リクエスト",
        ]
        let strings = [
            CalcScreenText.attackerHeading, CalcScreenText.defenderHeading, CalcScreenText.loading,
            CalcScreenText.resultsListLabel, CalcScreenText.disclosureOpened, CalcScreenText.disclosureClosed,
            AttackerStatLabels.spIncrease(.atk), AttackerStatLabels.spDecrease(.spa),
        ]
        for text in strings {
            XCTAssertFalse(text.isEmpty)
            for word in forbidden {
                XCTAssertFalse(text.contains(word), "「\(word)」は禁止語: \(text)")
            }
        }
    }
}
