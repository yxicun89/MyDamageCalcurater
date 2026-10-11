import SwiftUI
import WishlistCore

/// 商品の画像。与えられた枠いっぱいに出す(`fit` なら全体が収まるように)。ファイルキャッシュ経由で出し、
/// 取れなければジャンル色のプレースホルダー(画像が無くても成立する)。
struct ItemImageView: View {
    let item: Item
    /// ジャンルの色 `#rrggbb`(プレースホルダーの色)
    var genreColor: String = GenreIcon.defaultColor
    /// true: 画像全体を収める(詳細)。false: 枠いっぱいに切り抜く(ホーム)
    var fit = false

    @Environment(AppModel.self) private var model
    @State private var image: UIImage?

    var body: some View {
        Color.clear
            .overlay {
                if let image {
                    Image(uiImage: image).resizable().aspectRatio(contentMode: fit ? .fit : .fill)
                } else {
                    placeholder
                }
            }
            .clipped()
            .task(id: "\(item.imageURLPath)|\(model.settings.apiBaseURL ?? "")") { await load() }
    }

    private var placeholder: some View {
        let color = Color(hex: genreColor)
        return ZStack {
            color.opacity(0.14)
            Image(systemName: "gift").font(.system(size: 34)).foregroundStyle(color.opacity(0.8))
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
