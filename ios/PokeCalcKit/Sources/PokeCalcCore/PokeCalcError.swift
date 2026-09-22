// PokeCalcError: PokeCalcService の唯一のエラー型(ADR-0500 §3)。
// 画面はこの1種類だけを扱えばよい(通信失敗・サーバーのエラー応答・クライアント側の
// 制約のどれでもここに写す)。

/// `PokeCalcService` が投げる唯一のエラー。`code` は openapi `Error.code` をそのまま運ぶ
/// (サーバーが決めた語彙)。クライアント側で作るコードは `Code` にまとめ、
/// サーバーの語彙と衝突しない接頭辞を付ける。
public struct PokeCalcError: Error, Equatable, Sendable {
    public let code: String
    public let message: String

    public init(code: String, message: String) {
        self.code = code
        self.message = message
    }

    /// クライアント側で作るエラーコード(1か所に集約。coding-rules §2)。
    public enum Code {
        /// 通信できない(接続失敗・タイムアウト等)。HTTP 応答が無い失敗すべて。
        public static let transport = "client_transport_error"
        /// 応答は受け取ったが期待した形の JSON にデコードできない。`transport`(接続できない)とは
        /// 原因が違うので分ける。
        public static let decode = "client_decode_error"

        // MARK: - CalcViewModel が使う値(P6-2a)

        /// `AttackerPreset.build` に要る性格(上昇 or 無補正)がマスタの一覧に無い(ADR-0500 §6)。
        public static let natureUnavailable = "client_nature_unavailable"
        /// 計算画面に攻撃側・防御側として選べる種族が2つ未満(マスタが少なすぎる)。
        public static let insufficientSpecies = "client_insufficient_species"
        /// 攻撃側の learnset とマスタの技を突き合わせても、選べる技が1つも無い
        /// (データ由来。`reselectMove` が既定技を選べないときに使う)。
        public static let moveUnavailable = "client_move_unavailable"
        /// 選択中の `moveId` が `moveOptions` に無い(内部の不整合。`moveUnavailable` とは原因が違う。
        /// `moveOptions` にある技だけを選べるはずなので、通常は起きない防御的なエラー)。
        public static let selectedMoveMissing = "client_selected_move_missing"

        // MARK: - MockPokeCalcService が使う値

        /// サーバーの語彙を真似た「見つからない」。`APIPokeCalcServiceTests` のフィクスチャで
        /// 実サーバーが返す例として使っている値と同じにする(species 404 の code)。
        /// モックはサーバーではないが、画面が実装(API/モック)を問わず同じ `code` で分岐できるように
        /// 同じ語彙を選ぶ。
        public static let notFound = "not_found"
        /// サーバーの語彙を真似た「入力が不正」。WASM 境界のエラー語彙 `engine/wasmapi` の
        /// `CodeInvalidInput`、および openapi `ErrorCode.invalid_input` と同じ値
        /// (HTTP と WASM で code を共通化する。ADR-0200)。
        public static let invalidInput = "invalid_input"
        /// モックのフィクスチャ(`Resources/*.json`)自体が読み込めない。サーバーには無い、
        /// クライアント(モック実装)だけの内部エラーなので `client_` 接頭辞を付ける。
        public static let fixtureMissing = "client_fixture_missing"
        /// モックのフィクスチャ JSON の値が想定した形(タイプ・分類・ステータスの enum 文字列)でない。
        public static let fixtureInvalid = "client_fixture_invalid"

        // MARK: - TeamValidator / TeamStore が使う値(P6-2c。ADR-0501「P6-2c」1章)

        /// `Team.name` が空(前後空白を落として空になる場合を含む)。
        public static let teamNameEmpty = "client_team_name_empty"
        /// `Team.members` が `TeamLimits.maxMembers` を超える。
        public static let teamTooManyMembers = "client_team_too_many_members"
        /// メンバー1体の `moveIds` が `TeamLimits.maxMovesPerMember` を超える。
        public static let teamTooManyMoves = "client_team_too_many_moves"
        /// メンバー1体の `moveIds` に重複がある。
        public static let teamDuplicateMoves = "client_team_duplicate_moves"
        /// メンバー1体の SP が `SPLimits.maxPerStat`(単体)または `SPLimits.maxTotal`(合計)を超える。
        public static let teamSPInvalid = "client_team_sp_invalid"
        /// `LocalTeamStore` の保存データが JSON として `[Team]` にデコードできない。
        public static let teamStoreCorrupted = "client_team_store_corrupted"
    }
}
