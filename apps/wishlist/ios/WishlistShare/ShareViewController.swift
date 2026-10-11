import SwiftUI
import UIKit
import UniformTypeIdentifiers
import WishlistCore

/// Share Extension のエントリ(共有シートから「欲しいもの」を選ぶと開く)。
/// 本体アプリとデータを共有せず(仕様 §9.5)、拡張自身が API へ直接 POST する(接続先はビルド設定)。
/// Safari から共有したときは、`ExtensionPreprocessing.js` がページから読んだ url・title・imageURL を使う。
final class ShareViewController: UIViewController {
    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        Task { @MainActor in
            let page = await Self.sharedPage(from: extensionContext)
            let host = UIHostingController(
                rootView: ShareView(
                    sharedPage: page,
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

    /// 共有された項目から、JavaScript の結果(`NSExtensionJavaScriptPreprocessingResultsKey`)を `SharedPage` にする。
    /// 結果が無い(Safari 以外・JavaScript の失敗)ときは、URL / テキストから取り出した URL だけで `SharedPage` を作る。
    private static func sharedPage(from context: NSExtensionContext?) async -> SharedPage {
        let providers = (context?.inputItems as? [NSExtensionItem] ?? []).flatMap { $0.attachments ?? [] }
        for provider in providers where provider.hasItemConformingToTypeIdentifier(UTType.propertyList.identifier) {
            if let item = try? await provider.loadItem(forTypeIdentifier: UTType.propertyList.identifier),
                let dictionary = item as? [String: Any],
                let results = dictionary[NSExtensionJavaScriptPreprocessingResultsKey] as? [String: Any]
            {
                let page = SharedPage.parse(results)
                if page.url != nil { return page }
            }
        }
        return SharedPage(url: await sharedURL(from: providers))
    }

    /// 共有された項目から URL を取り出す。Safari は URL、他のアプリはテキストで渡してくる(`SharedURL.extract`)。
    private static func sharedURL(from providers: [NSItemProvider]) async -> URL? {
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
