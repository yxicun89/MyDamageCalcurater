import PokeCalcCore
import SwiftUI

// ポケモン画像(P8-1c。ADR-0508)。manifest にキーがあり読み込みに成功したときだけ画像、
// 読み込み中・失敗・キー無しはタイプ色エンブレム(画像は必須にしない)。アニメーション・fade-in は付けない。

private struct ImageCatalogKey: EnvironmentKey {
    static let defaultValue: any ImageCatalog = NoImageCatalog()
}

extension EnvironmentValues {
    /// 画像の問い合わせ先。既定は画像なし(プレビュー・既存テスト)。`RootView` が起動時のものを注入する。
    var imageCatalog: any ImageCatalog {
        get { self[ImageCatalogKey.self] }
        set { self[ImageCatalogKey.self] = newValue }
    }
}

/// `SpeciesEmblemView` の置き換え先。エンブレムと同じ直径の円に画像を切り抜く(文字サイズで大きくならない)。
/// `fallback` は画像が無い・読み込み中・失敗のときの表示(既定はタイプ色エンブレム。素早さ表などは小さい直径の自前エンブレム)。
struct SpeciesImageView<Fallback: View>: View {
    let speciesKey: String?
    let diameter: CGFloat
    private let fallback: Fallback

    @Environment(\.imageCatalog) private var catalog
    @State private var url: URL?

    init(speciesKey: String?, diameter: CGFloat = SpeciesEmblemView.diameter, @ViewBuilder fallback: () -> Fallback) {
        self.speciesKey = speciesKey
        self.diameter = diameter
        self.fallback = fallback()
    }

    var body: some View {
        content
            .task(id: speciesKey) {
                // キーが変わったら、新しい引き当てが済むまで前のポケモンの画像を残さない(エンブレムに戻す)。
                url = nil
                guard let key = SpeciesImageDisplay.lookupKey(speciesKey: speciesKey) else { return }
                let resolved = await catalog.imageURL(speciesKey: key, size: .thumb)
                // 待っている間にキーが変わっていたら捨てる(ステイル応答)。
                guard !Task.isCancelled else { return }
                url = resolved
            }
    }

    @ViewBuilder
    private var content: some View {
        if let url, let key = SpeciesImageDisplay.lookupKey(speciesKey: speciesKey) {
            AsyncImage(url: url) { phase in
                switch phase {
                case .success(let image):
                    image
                        .resizable()
                        .scaledToFill()
                        .frame(width: diameter, height: diameter)
                        .clipShape(Circle())
                        // 装飾(名前は隣のテキストが読む)。label は付けず、identifier だけで枠を公開する。
                        .accessibilityElement(children: .ignore)
                        .accessibilityIdentifier("speciesImage-\(key)")
                case .empty, .failure:
                    fallback
                @unknown default:
                    fallback
                }
            }
        } else {
            fallback
        }
    }
}

extension SpeciesImageView where Fallback == SpeciesEmblemView {
    init(speciesKey: String?, name: String, types: [PokeType]) {
        self.init(speciesKey: speciesKey) { SpeciesEmblemView(name: name, types: types) }
    }
}
