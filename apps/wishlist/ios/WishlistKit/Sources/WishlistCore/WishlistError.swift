import Foundation

/// API・通信・キャッシュ層が投げるエラー。code は api/openapi.yaml の Error.code に、通信できなかったときの `network` と、
/// 応答を読めなかったときの `decode` を足したもの(web の ApiError と同じ考え方)。
public struct WishlistError: Error, Equatable, Sendable {
    public enum Code: String, Sendable, Equatable {
        case badRequest = "bad_request"
        case unauthorized
        case notFound = "not_found"
        case unprocessable
        case badGateway = "bad_gateway"
        case notImplemented = "not_implemented"
        case `internal`
        case network
        case decode
    }

    public var code: Code
    /// HTTP ステータス。通信できなかったときは 0
    public var status: Int
    public var message: String

    public init(code: Code, status: Int = 0, message: String) {
        self.code = code
        self.status = status
        self.message = message
    }

    public var isNetwork: Bool { code == .network }

    /// spec-writer のスタブ用(実装が済んだら使われなくなる)。
    public static let stub = WishlistError(code: .internal, message: "not implemented (spec stub)")
}
