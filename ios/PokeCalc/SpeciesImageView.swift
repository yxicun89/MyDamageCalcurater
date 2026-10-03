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
struct SpeciesImageView: View {
    let speciesKey: String?
    let name: String
    let types: [PokeType]

    @Environment(\.imageCatalog) private var catalog
    @State private var url: URL?

    var body: some View {
        content
            .task(id: speciesKey) {
                // キーが変わったら、新しい引き当てが済むまで前のポケモンの画像を残さない(エンブレムに戻す)。
                url = nil
                guard let key = SpeciesImageDisplay.lookupKey(speciesKey: speciesKey) else {
                    url = nil
                    return
                }
                url = await catalog.imageURL(speciesKey: key, size: .thumb)
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
                        .frame(width: SpeciesEmblemView.diameter, height: SpeciesEmblemView.diameter)
                        .clipShape(Circle())
                        // 装飾(名前は隣のテキストが読む)。label は付けず、identifier だけで枠を公開する。
                        .accessibilityElement(children: .ignore)
                        .accessibilityIdentifier("speciesImage-\(key)")
                case .empty, .failure:
                    SpeciesEmblemView(name: name, types: types)
                @unknown default:
                    SpeciesEmblemView(name: name, types: types)
                }
            }
        } else {
            SpeciesEmblemView(name: name, types: types)
        }
    }
}
