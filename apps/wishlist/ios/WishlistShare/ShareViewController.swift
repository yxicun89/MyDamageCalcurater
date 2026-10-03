import SwiftUI
import UIKit
import UniformTypeIdentifiers
import WishlistCore

/// Share Extension のエントリ(共有シートから「欲しいもの」を選ぶと開く)。
/// 本体アプリとデータを共有せず(仕様 §9.5)、拡張自身の UserDefaults の接続設定で API へ直接 POST する。
final class ShareViewController: UIViewController {
    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        Task { @MainActor in
            let url = await Self.sharedURL(from: extensionContext)
            let host = UIHostingController(
                rootView: ShareView(
                    sharedURL: url,
                    onFinish: { [weak self] in self?.extensionContext?.completeRequest(returningItems: nil) },
                    onCancel: { [weak self] in
                        self?.extensionContext?.cancelRequest(withError: NSError(domain: NSCocoaErrorDomain, code: NSUserCancelledError))
                    }))
            addChild(host)
            host.view.frame = view.bounds
            host.view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
            view.addSubview(host.view)
            host.didMove(toParent: self)
        }
    }

    /// 共有された項目から URL を取り出す。Safari は URL、他のアプリはテキストで渡してくる(`SharedURL.extract`)。
    private static func sharedURL(from context: NSExtensionContext?) async -> URL? {
        let providers = (context?.inputItems as? [NSExtensionItem] ?? []).flatMap { $0.attachments ?? [] }
        for provider in providers where provider.hasItemConformingToTypeIdentifier(UTType.url.identifier) {
            if let item = try? await provider.loadItem(forTypeIdentifier: UTType.url.identifier) {
                if let url = item as? URL, Deeplink.isHTTPURL(url.absoluteString) { return url }
                if let text = item as? String, let url = SharedURL.extract(from: text) { return url }
            }
        }
        for provider in providers where provider.hasItemConformingToTypeIdentifier(UTType.plainText.identifier) {
            if let item = try? await provider.loadItem(forTypeIdentifier: UTType.plainText.identifier),
                let text = item as? String, let url = SharedURL.extract(from: text)
            {
                return url
            }
        }
        return nil
    }
}
