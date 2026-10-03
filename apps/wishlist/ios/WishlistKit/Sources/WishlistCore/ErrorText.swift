import Foundation

/// 画面に出すエラーの文言(WishlistError は API のメッセージ、それ以外は localizedDescription)。
func errorText(_ error: any Error) -> String {
    if let error = error as? WishlistError {
        return error.isNetwork ? "通信できません(\(error.message))" : error.message
    }
    return error.localizedDescription
}
