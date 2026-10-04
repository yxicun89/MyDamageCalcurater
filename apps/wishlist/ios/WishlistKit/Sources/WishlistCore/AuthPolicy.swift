import Foundation

/// Bearer トークンを付けてよい URL かの判定(API クライアントと画像取得が同じ判定を使う)。
/// 設定したベース URL と scheme・host・port が同じで、パスがベース URL の配下のときだけ true。
/// 画像の URL は絶対 URL のこともある(外部ホスト)ので、そこへトークンを送らないための境界。
public enum AuthPolicy {
    public static func shouldAttachToken(url: URL, baseURL: URL) -> Bool {
        guard let scheme = url.scheme?.lowercased(), scheme == baseURL.scheme?.lowercased(),
            let host = url.host?.lowercased(), host == baseURL.host?.lowercased(),
            port(of: url, scheme: scheme) == port(of: baseURL, scheme: scheme)
        else { return false }
        let basePath = withTrailingSlash(baseURL.path)
        return withTrailingSlash(url.path).hasPrefix(basePath)
    }

    private static func port(of url: URL, scheme: String) -> Int? {
        url.port ?? (scheme == "https" ? 443 : scheme == "http" ? 80 : nil)
    }

    private static func withTrailingSlash(_ path: String) -> String {
        path.hasSuffix("/") ? path : path + "/"
    }
}

/// 画像を取る関数(`ImageFileCache` の loader)。ベース URL の配下にだけ Bearer を付け、外部 URL にはヘッダーを付けない。
public enum ImageLoader {
    public static func make(
        baseURL: URL, token: String, session: URLSession = .shared
    ) -> @Sendable (URL) async throws -> Data {
        { url in
            var request = URLRequest(url: url)
            if AuthPolicy.shouldAttachToken(url: url, baseURL: baseURL) {
                request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
            }
            let (data, response) = try await session.data(for: request)
            guard (response as? HTTPURLResponse)?.statusCode == 200 else { throw URLError(.badServerResponse) }
            return data
        }
    }
}
