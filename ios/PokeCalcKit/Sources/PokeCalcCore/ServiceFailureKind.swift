// ServiceFailureKind: サーバーが使えない理由の分類(ADR-0534)。全画面がこの 1 か所の分類と固定文を使う。
// 番号(HTTP)・code・サーバーの英語は画面に出さない(分類にだけ使う)。

/// 画面に伝える失敗の原因。
public enum ServiceFailureKind: Equatable, Sendable {
    /// 接続できない(圏外・ホストに届かない)。
    case offline
    /// 時間内に返事が無かった。
    case timeout
    /// サーバー側の障害(5xx・サーバーの内部エラー・保存先や取得元の不調)。
    case serverFault
    /// いまつないでいるサーバーがその機能を持っていない(経路の無い 404・501)。
    case unsupported
    /// 返事を読み取れない(形のずれ・番号の無い契約外の応答)。
    case unreadable
    /// サーバーが混み合っている。
    case overloaded
    /// 画面別の文言が要る失敗(入力の誤りなど)。
    case other(code: String)

    public init(_ error: any Error) {
        guard let error = error as? PokeCalcError else {
            self = .unreadable
            return
        }
        switch error.code {
        case PokeCalcError.Code.transport:
            self = .offline
        case PokeCalcError.Code.timeout:
            self = .timeout
        case "internal_error", "master_unavailable", "upstream_unavailable", "store_unavailable":
            self = .serverFault
        case "overloaded":
            self = .overloaded
        case PokeCalcError.Code.decode, PokeCalcError.Code.unexpectedStatus:
            // 契約どおりに読めなかった応答。番号があれば、ゲートウェイの HTML などの 5xx・経路の無い 404 を見分ける。
            self = Self.byStatus(error.statusCode) ?? .unreadable
        default:
            // 画面別の文言が要る失敗。ただしサーバー側の 5xx(契約の `internal` など)は障害として扱う。
            if let status = error.statusCode, (500...599).contains(status), status != 501 {
                self = .serverFault
            } else {
                self = .other(code: error.code)
            }
        }
    }

    /// 契約どおりに読めなかった応答の番号 → 分類。経路の無い 404・501 は「まだ使えない」、他の 5xx は「サーバーの障害」。
    private static func byStatus(_ status: Int?) -> ServiceFailureKind? {
        switch status ?? 0 {
        case 404, 501: return .unsupported
        case 500...599: return .serverFault
        default: return nil
        }
    }

    /// 画面に出す文言。`feature` は機能名(「計算」「素早さ」など)。
    public func message(feature: String) -> String {
        switch self {
        case .offline:
            return "サーバーに接続できません。インターネットの接続を確かめて、もう一度お試しください"
        case .timeout:
            return "サーバーから時間内に返事がありませんでした。しばらくしてからもう一度お試しください"
        case .serverFault:
            return "\(feature)のサーバーで問題が起きています。しばらくしてからもう一度お試しください"
        case .unsupported:
            return "\(feature)は、いまつないでいるサーバーではまだ使えません"
        case .unreadable:
            return "サーバーの返事を読み取れませんでした。アプリかサーバーの更新が必要かもしれません"
        case .overloaded:
            return "サーバーが混み合っています。しばらくしてからもう一度お試しください"
        case .other:
            return "うまくいきませんでした。もう一度お試しください"
        }
    }
}

extension ServiceFailureKind {
    /// `.other` の失敗に出す文言。アプリ側が日本語で書いた説明(`detail`)があればそれを、
    /// サーバーの英語・空なら汎用の文言を出す(コード・英語を画面に出さない。ADR-0534)。
    func message(feature: String, detail: String) -> String {
        if case .other = self, !detail.isEmpty, detail.range(of: "[A-Za-z]", options: .regularExpression) == nil {
            return detail
        }
        return message(feature: feature)
    }
}

/// 失敗まわりの固定文。
public enum ServiceFailureText {
    /// 接続先が空のビルド(`AppConfiguration.Backend.unconfigured`)と、接続先なしのタイプバランス。
    public static let notConfigured = "サーバーの接続先が設定されていません"
    /// 最初の読み込みに失敗した画面の再試行ボタン。
    public static let retryButtonLabel = "もう一度"
}
