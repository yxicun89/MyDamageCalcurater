import XCTest

@testable import PokeCalcCore

/// G-05(ADR-0536。docs/design.md「見た目の作り直し方針(G-05)」・Web の ADR-0339/0344/0348/0349)。
/// 「このアプリについて」「お気に入り・履歴」「お気に入りから読み込む」「お気に入りに追加」の画面に出す文字を、
/// 「名前 / 短いラベル / 数値・結果 / 状態の一言」の 4 種に絞る。文を置いてよいのは、エラー・空の状態・
/// 取り消せない操作の確認・単位が分からない欄・「このアプリについて」だけ(design.md「文を減らす決まり」)。
/// 補足の文は「説明」ボタンの奥(`CommonPartsText.help`)へ移す。
final class G05AboutFavoritesTextTests: XCTestCase {

    // MARK: - 共通の部品の語(docs/glossary.md「共通の部品の語」)

    func testCommonPartsWordsMatchGlossary() {
        XCTAssertEqual(CommonPartsText.help, "説明")
        XCTAssertEqual(CommonPartsText.close, "閉じる")
    }

    // MARK: - このアプリについて(Web の aboutText と同じ見出し。文面は変えない)

    func testAboutHeadingsMatchWeb() {
        XCTAssertEqual(AboutText.unofficialHeading, "非公式表示")
        XCTAssertEqual(AboutText.dataSourcesHeading, "データの出典")
        XCTAssertEqual(DeviceDataText.sectionTitle, "データの扱い")
    }

    func testAboutBodyTextsAreKeptAsIs() {
        // 例外の画面なので文面は変えない(Web の決定 2026-10-11-web-g05-about)。
        XCTAssertEqual(AboutText.dataSources.count, 4)
        XCTAssertEqual(AboutText.unofficialNotice.filter { $0 == "。" }.count, 3)
    }

    func testDeviceDataExplanationBulletsAreOneSentenceEach() {
        // 「データの扱い」の説明(3 要素・各 2 文。文面は変えない)を、1 文ずつの箇条で描く(Web と同じ)。
        let bullets = DeviceDataText.explanationBullets
        XCTAssertEqual(bullets.count, 6)
        XCTAssertEqual(bullets.joined(), DeviceDataText.explanation.joined(), "文面は 1 文字も変えず、順序もそのまま")
        for sentence in bullets {
            XCTAssertEqual(sentence.filter { $0 == "。" }.count, 1, "1 文ずつ: \(sentence)")
            XCTAssertTrue(sentence.hasSuffix("。"), sentence)
        }
    }

    // MARK: - お気に入り・履歴: 空の状態は一言(行動のボタンは画面側)

    func testEmptyStatesAreOneShortLine() {
        XCTAssertEqual(FavoritesLabels.emptyFavorites, "お気に入りはまだありません")
        XCTAssertEqual(CalcHistoryLabels.emptyHistory, "計算履歴はまだありません")
        XCTAssertEqual(FavoritesLabels.emptyHistory, "計算した相手がまだありません")
    }

    // MARK: - お気に入りの行: 短い文字 + 件数は数値

    func testRowAndCountWords() {
        XCTAssertEqual(FavoritesLabels.useButton, "使う")
        XCTAssertEqual(FavoritesLabels.removeButton, "削除")
        XCTAssertEqual(FavoritesLabels.countText(3), "3/\(RequestLimits.maxFavorites)件")
        XCTAssertEqual(FavoritesLabels.countText(3), "3/100件")
        XCTAssertEqual(FavoritesLabels.legacyFavoriteHint, "個体だけ(計算の条件なし)")
    }

    func testAccessibilityLabelsKeepVisibleWords() {
        // 見える文字を名前に含める(WCAG 2.5.3)。読み上げ名は Web と同じ形。
        XCTAssertEqual(FavoritesLabels.useAccessibilityLabel(title: "雨で攻撃"), "「雨で攻撃」を計算に使う")
        XCTAssertTrue(FavoritesLabels.useAccessibilityLabel(title: "雨で攻撃").contains(FavoritesLabels.useButton))
        XCTAssertEqual(FavoritesLabels.removeAccessibilityLabel(title: "雨で攻撃"), "「雨で攻撃」を削除")
        XCTAssertTrue(
            FavoritesLabels.removeAccessibilityLabel(title: "雨で攻撃").contains(FavoritesLabels.removeButton))
    }

    // MARK: - 削除は 2 段階(取り消せない操作の確認は文を置いてよい)

    func testRemoveConfirmationWords() {
        XCTAssertEqual(
            FavoritesLabels.removeConfirmMessage(title: "HB特化"), "「HB特化」をお気に入りから外します。よろしいですか?")
        XCTAssertEqual(FavoritesLabels.removeConfirmAction, "削除する")
        XCTAssertEqual(FavoritesLabels.removeCancelAction, "やめる")
    }

    // MARK: - お気に入りから読み込む

    func testLoadEntryTitlesAreShort() {
        XCTAssertEqual(FavoritesLabels.loadTitle(for: .attacker), "攻撃側のお気に入り")
        XCTAssertEqual(FavoritesLabels.loadTitle(for: .defender), "防御側のお気に入り")
        for side in [FavoriteLoadSide.attacker, .defender] {
            XCTAssertLessThanOrEqual(FavoritesLabels.loadTitle(for: side).count, 10, "短いラベル(目安 8 文字。名前つきで 10 まで)")
            XCTAssertFalse(FavoritesLabels.loadTitle(for: side).contains("。"))
        }
    }

    func testLoadNotesAreKeptForTheHelpPopover() {
        // 注記は画面に出し続けず「説明」ボタンの奥へ移すが、文そのものは変えない(2 文以内)。
        XCTAssertEqual(FavoritesLabels.loadNote(for: .attacker).filter { $0 == "。" }.count, 2)
        XCTAssertEqual(FavoritesLabels.loadNote(for: .defender).filter { $0 == "。" }.count, 2)
        XCTAssertEqual(CalcHistoryLabels.note.filter { $0 == "。" }.count, 2)
        XCTAssertEqual(FavoritesLabels.restoreLimitNote.filter { $0 == "。" }.count, 2)
    }

    // MARK: - 画面に常に出す文字に「。」を置かない(文を減らす決まり)

    /// 常に画面に出る「名前・短いラベル・状態の一言」。文(句点)にしない。
    func testAlwaysVisibleStringsAreNotSentences() {
        let visible: [(String, String)] = [
            ("FavoritesLabels.screenTitle", FavoritesLabels.screenTitle),
            ("favoritesSectionTitle", FavoritesLabels.favoritesSectionTitle),
            ("historySectionTitle", FavoritesLabels.historySectionTitle),
            ("emptyFavorites", FavoritesLabels.emptyFavorites),
            ("emptyHistory", FavoritesLabels.emptyHistory),
            ("CalcHistoryLabels.emptyHistory", CalcHistoryLabels.emptyHistory),
            ("removeButton", FavoritesLabels.removeButton),
            ("useButton", FavoritesLabels.useButton),
            ("retryButton", FavoritesLabels.retryButton),
            ("legacyFavoriteHint", FavoritesLabels.legacyFavoriteHint),
            ("countText", FavoritesLabels.countText(12)),
            ("loadTitle.attacker", FavoritesLabels.loadTitle(for: .attacker)),
            ("loadTitle.defender", FavoritesLabels.loadTitle(for: .defender)),
            ("loadCloseButton", FavoritesLabels.loadCloseButton),
            ("loadLoading", FavoritesLabels.loadLoading),
            ("saving", FavoritesLabels.saving),
            ("loadMoreButton", CalcHistoryLabels.loadMoreButton),
            ("loadingMore", CalcHistoryLabels.loadingMore),
            ("AboutText.unofficialHeading", AboutText.unofficialHeading),
            ("AboutText.dataSourcesHeading", AboutText.dataSourcesHeading),
            ("CommonPartsText.help", CommonPartsText.help),
            ("CommonPartsText.close", CommonPartsText.close),
        ]
        for (name, text) in visible {
            XCTAssertFalse(text.isEmpty, "\(name) が空")
            XCTAssertFalse(text.contains("。"), "\(name) は文にしない: \(text)")
            XCTAssertFalse(text.contains("\n"), name)
            XCTAssertNil(text.range(of: "[A-Za-z_]{3,}", options: .regularExpression), "\(name): 英字の code を出さない")
        }
    }

    /// 状態の一言(追加の結果)は 1 文まで。
    func testPinStatusLinesAreAtMostOneSentence() {
        for text in [FavoritesLabels.pinned, FavoritesLabels.alreadyPinned] {
            XCTAssertLessThanOrEqual(text.filter { $0 == "。" }.count, 1, text)
        }
    }
}
