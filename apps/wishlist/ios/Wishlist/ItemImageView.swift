import SwiftUI
import WishlistCore

/// 商品の画像(正方形に切り抜く)。ファイルキャッシュ経由で出し、取れなければ色付きのプレースホルダー(画像が無くても成立する)。
struct ItemImageView: View {
    let item: Item
    var cornerRadius: CGFloat = 14

    @Environment(AppModel.self) private var model
    @State private var image: UIImage?

    var body: some View {
        Color.clear
            .aspectRatio(1, contentMode: .fit)
            .overlay {
                if let image {
                    Image(uiImage: image).resizable().scaledToFill()
                } else {
                    placeholder
                }
            }
            .clipShape(RoundedRectangle(cornerRadius: cornerRadius, style: .continuous))
            .task(id: "\(item.imageURLPath)|\(model.settings.apiBaseURL ?? "")") { await load() }
    }

    private var placeholder: some View {
        // id から決まる色(タイプ色エンブレムの代わり。常時動かない)
        let hue = Double((item.id * 47) % 360) / 360
        return ZStack {
            Color(hue: hue, saturation: 0.35, brightness: 0.85)
            Image(systemName: "gift").font(.title).foregroundStyle(.white.opacity(0.8))
        }
    }

    private func load() async {
        guard let base = model.settings.baseURL, let url = WishlistURLs.imageURL(path: item.imageURLPath, baseURL: base) else { return }
        let cache = model.imageCache
        if let data = cache.cachedData(for: url), let saved = UIImage(data: data) {
            image = saved
            return
        }
        if let data = try? await cache.data(for: url) { image = UIImage(data: data) }
    }
}
