// AboutText: 「このアプリについて」画面(P6-18)の固定文言。
//
// マスタ(pokedex)には無い表示専用の文言なので、`DisplayLabels.swift` と同じ理由でコードに1か所持つ
// (coding-rules §2「ハードコードしない」の対象は使用可能なポケモン・技・持ち物・タイプ相性のような
// レギュレーション依存のマスタデータであり、固定の日本語文言はここに置いてよい)。
// Web レーンも同じ語を使う(issue #328。docs/ai-shared/DECISIONS.md「P6-18」参照)。
//
// 出典の一覧は docs/adr/0002-master-data-source.md「確定した方針 / 責務の分離」表と一致させる
// (ADR-0501「P6-18」3章)。名称・ライセンスは同 ADR に書かれている範囲を超えて断定しない。

public enum AboutText {
    /// 非公式であることの注記(ADR-0501「P6-18」2章)。issue #328 のユーザー決定
    /// (「アプリ内に第三者データの出典と非公式の表示を入れる」)への回答。
    public static let unofficialNotice: String = {
        "このアプリは個人が私的に使うための非公式ツールです。"
            + "任天堂・クリーチャーズ・ゲームフリーク・株式会社ポケモンとは関係ありません。"
            + "ポケモン・Pokémon および関連する名称は各社の商標です。"
    }()

    /// データの出典1件(用途と出典・ライセンス)。
    public struct DataSource: Equatable, Sendable {
        /// 用途(何のために使っているか)。
        public let title: String
        /// 出典の名前とライセンス(不明なものはライセンスを書かない)。
        public let detail: String

        public init(title: String, detail: String) {
            self.title = title
            self.detail = detail
        }
    }

    /// データの出典一覧(ADR-0002「確定した方針 / 責務の分離」表と同じ順)。
    public static let dataSources: [DataSource] = {
        [
            DataSource(title: "ダメージ計算の検証", detail: "@smogon/calc(MIT License)"),
            DataSource(title: "ポケモン・技・習得技の照合", detail: "Pokémon Showdown(MIT License)"),
            DataSource(title: "日本語名・図鑑番号", detail: "PokeAPI"),
            DataSource(title: "使用可能なポケモン等の基準", detail: "Pokémon HOME・Pokémon Champions の公式情報"),
        ]
    }()
}
