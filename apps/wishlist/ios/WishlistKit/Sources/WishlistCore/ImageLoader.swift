import Foundation

/// 画像を取る関数(`ImageFileCache` の loader)。認証の情報は付けない。
public enum ImageLoader {
    public static func make(session: URLSession = .shared) -> @Sendable (URL) async throws -> Data {
        { url in
            let (data, response) = try await session.data(for: URLRequest(url: url))
            guard (response as? HTTPURLResponse)?.statusCode == 200 else { throw URLError(.badServerResponse) }
            return data
        }
    }
}
