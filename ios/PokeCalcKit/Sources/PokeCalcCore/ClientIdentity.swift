import Foundation

/// 端末 ID とセッション ID(CLAUDE.md 技術規約「クライアントは端末ID(UUID)とセッションIDを
/// 全リクエストに付与する」。ADR-0017 §5)。
///
/// 端末 ID は初回起動で作って `UserDefaults` に保存し、以後は同じ値を使う(秘密ではない)。
/// セッション ID は起動(インスタンス)ごとに新しく作る。
public struct ClientIdentity: Sendable {
    public static let deviceIDDefaultsKey = "PokeCalcDeviceID"

    public let deviceID: String
    public let sessionID: String

    /// テストや `APIPokeCalcService` へ固定値を渡すための初期化子。
    public init(deviceID: String, sessionID: String) {
        self.deviceID = deviceID
        self.sessionID = sessionID
    }

    /// `defaults` から端末 ID を読み、無い・壊れている(UUID でない)ときは作って保存し直す。
    public init(defaults: UserDefaults) {
        let stored = defaults.string(forKey: Self.deviceIDDefaultsKey)
        if let stored, UUID(uuidString: stored) != nil {
            self.deviceID = stored
        } else {
            let created = UUID().uuidString
            defaults.set(created, forKey: Self.deviceIDDefaultsKey)
            self.deviceID = created
        }
        self.sessionID = UUID().uuidString
    }
}
