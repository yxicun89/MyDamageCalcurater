import Foundation

// ポケモン画像(P8-1c。ADR-0508・ADR-0807)。足場: 全部「画像なし」を返す。
// 画像が無い(manifest 404・不正・version 違い・キー無し)ときは必ず「画像なし」= 既存のタイプ色エンブレムにする。
// throw しない・画面を待たせない。

/// 画像の大きさ種別。`ImageManifest` のキー名と同じ(`thumb`・`detail`)。
public enum PokeImageSize: String, Sendable, CaseIterable {
    case thumb
    case detail
}

/// `/images/manifest.json` の写し。`{"version":1,"images":{"0445-000":{"thumb":"...","detail":"..."}}}`。
/// version が 1 以外・不正 JSON・形の違いは throw せず `.empty`(画像なし)にする。未知の欄は無視する。
public struct ImageManifest: Equatable, Sendable {
    public static let supportedVersion = 1
    public static let empty = ImageManifest(entries: [:])

    /// キー(speciesKey)→ サイズ → `/images/` からの相対パス。
    public let entries: [String: [PokeImageSize: String]]

    public init(entries: [String: [PokeImageSize: String]]) {
        self.entries = entries
    }

    /// JSON を decode する(上記の規則)。1エントリ・1サイズの型違いはそこだけ捨てる。
    public init(decoding data: Data) {
        guard let root = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
            Self.isSupportedVersion(root["version"]),
            let images = root["images"] as? [String: Any]
        else {
            self = .empty
            return
        }
        var entries: [String: [PokeImageSize: String]] = [:]
        for (key, value) in images {
            guard let sizes = value as? [String: Any] else { continue }
            var paths: [PokeImageSize: String] = [:]
            for size in PokeImageSize.allCases {
                if let path = sizes[size.rawValue] as? String { paths[size] = path }
            }
            if !paths.isEmpty { entries[key] = paths }
        }
        self.init(entries: entries)
    }

    /// 数値の 1 だけ許す(文字列・null・真偽値・1.5 は不可)。
    private static func isSupportedVersion(_ value: Any?) -> Bool {
        guard let number = value as? NSNumber, CFGetTypeID(number) != CFBooleanGetTypeID() else { return false }
        return number.doubleValue == Double(supportedVersion)
    }

    /// そのキー・サイズの相対パス。無ければ nil(画像なし)。
    public func relativePath(forKey key: String, size: PokeImageSize) -> String? {
        entries[key]?[size]
    }
}

/// 基点 URL + `/images/` + 相対パス。危険・不正な相対パスは nil(画像なし)。
public enum PokeImageURL {
    /// 拒否: 空・先頭 `/`・`..` や空のセグメント・scheme(`:`)・`//host`・制御文字・`?` `#` `\\` `%` を含む。
    /// 基点 URL の末尾スラッシュ有無・パスの有無(`http://h/prefix`)の両方で `/images/` を1本だけ挟む。
    public static func url(baseURL: URL, relativePath: String) -> URL? {
        guard !relativePath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
            !relativePath.hasPrefix("/"),
            !relativePath.unicodeScalars.contains(where: { forbiddenScalars.contains($0) })
        else { return nil }
        let segments = relativePath.split(separator: "/", omittingEmptySubsequences: false)
        guard !segments.contains(where: { $0.isEmpty || $0 == ".." || $0 == "." }) else { return nil }
        guard var components = URLComponents(url: baseURL, resolvingAgainstBaseURL: false) else { return nil }
        var basePath = components.path
        while basePath.hasSuffix("/") { basePath.removeLast() }
        components.path = "\(basePath)/images/\(relativePath)"
        components.query = nil
        components.fragment = nil
        return components.url
    }

    private static let forbiddenScalars: CharacterSet = {
        var set = CharacterSet.controlCharacters
        set.insert(charactersIn: "?#\\%:")
        return set
    }()
}

/// manifest 取得の1回の結果(テストで差し替える)。
public struct ImageManifestResponse: Sendable, Equatable {
    public let statusCode: Int
    public let data: Data

    public init(statusCode: Int, data: Data) {
        self.statusCode = statusCode
        self.data = data
    }
}

public protocol ImageManifestFetching: Sendable {
    /// 通信失敗は throw してよい(`ImageCatalog` 側が「画像なし」にする)。
    func fetchManifest(from url: URL) async throws -> ImageManifestResponse
}

/// 画面が引く入口。`nil` = 画像なし(エンブレムにする)。呼び出しは throw しない・待たせない。
public protocol ImageCatalog: Sendable {
    func imageURL(speciesKey: String, size: PokeImageSize) async -> URL?
}

/// API 実装。manifest を最初の引き当て時に1回だけ取得して保持する(起動時1回・手動更新なし。成功・失敗とも保持し再取得しない)。
/// 同時に複数の引き当てが来ても取得は1回。X-Device-Id は付けない(ADR-0807 決定 4)。
public actor RemoteImageCatalog: ImageCatalog {
    private let baseURL: URL
    private let fetcher: any ImageManifestFetching
    /// 進行中(または完了済み)の取得。成功も失敗も保持して再取得しない。
    private var loading: Task<ImageManifest, Never>?

    public init(baseURL: URL, fetcher: any ImageManifestFetching) {
        self.baseURL = baseURL
        self.fetcher = fetcher
    }

    /// `baseURL/images/manifest.json` を取得 → 200 以外・通信失敗は `.empty` →
    /// `ImageManifest(decoding:)` → `PokeImageURL.url` で組む。
    public func imageURL(speciesKey: String, size: PokeImageSize) async -> URL? {
        let manifest = await loadManifest()
        guard let path = manifest.relativePath(forKey: speciesKey, size: size) else { return nil }
        return PokeImageURL.url(baseURL: baseURL, relativePath: path)
    }

    private func loadManifest() async -> ImageManifest {
        if let loading { return await loading.value }
        let fetcher = fetcher
        let manifestURL = baseURL.appendingPathComponent("images").appendingPathComponent("manifest.json")
        let task = Task<ImageManifest, Never> {
            guard let response = try? await fetcher.fetchManifest(from: manifestURL), response.statusCode == 200 else {
                return .empty
            }
            return ImageManifest(decoding: response.data)
        }
        loading = task
        return await task.value
    }
}

/// URLSession 実装(`RemoteImageCatalog` の既定の取得手段)。ヘッダーは足さない。
public struct URLSessionImageManifestFetcher: ImageManifestFetching {
    private let session: URLSession

    public init(session: URLSession = .shared) {
        self.session = session
    }

    /// GET。HTTPURLResponse でなければ throw。
    public func fetchManifest(from url: URL) async throws -> ImageManifestResponse {
        let (data, response) = try await session.data(from: url)
        guard let http = response as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        return ImageManifestResponse(statusCode: http.statusCode, data: data)
    }
}

/// 画像なし(既定。モックの既定でもある)。
public struct NoImageCatalog: ImageCatalog {
    public init() {}

    public func imageURL(speciesKey: String, size: PokeImageSize) async -> URL? { nil }
}

public enum MockImageScenario: Equatable, Sendable {
    /// 画像なし。環境変数なし・未知の値の既定(既存のテスト・XCUITest は無変更で通る)。
    case none
    /// 9001-000・9003-000 だけ画像あり(data URL の小さな架空 PNG)。9002-000 などは画像なし。
    case available

    /// 環境変数の値: `1` が `.available`。nil・それ以外は `.none`。
    public init(environmentValue: String?) {
        self = environmentValue == "1" ? .available : .none
    }
}

public struct MockImageCatalog: ImageCatalog {
    public static let scenarioEnvironmentKey = "POKECALC_MOCK_IMAGES"
    /// `.available` で画像が出るキー(架空の 9xxx のみ)。
    public static let availableKeys: Set<String> = ["9001-000", "9003-000"]

    private let scenario: MockImageScenario

    public init(scenario: MockImageScenario = .none) {
        self.scenario = scenario
    }

    public init(environment: [String: String]) {
        self.init(scenario: MockImageScenario(environmentValue: environment[Self.scenarioEnvironmentKey]))
    }

    /// `.available` かつ `availableKeys` のとき、小さな架空 PNG の data URL(コードで生成)。
    public func imageURL(speciesKey: String, size: PokeImageSize) async -> URL? {
        guard scenario == .available, Self.availableKeys.contains(speciesKey) else { return nil }
        return Self.fictionalPNGDataURL
    }

    /// 8x8 の単色 PNG(無圧縮 deflate)。実画像ではなくコードで作る架空データ。
    private static let fictionalPNGDataURL: URL? = {
        let side: UInt32 = 8
        var raw = Data()
        for _ in 0..<side {
            raw.append(0)  // filter: none
            for _ in 0..<side { raw.append(contentsOf: [0x6B, 0x8E, 0xC8]) }  // RGB
        }
        var zlib = Data([0x78, 0x01, 0x01])
        let length = UInt16(raw.count)
        zlib.append(contentsOf: [UInt8(length & 0xFF), UInt8(length >> 8), UInt8(~length & 0xFF), UInt8(~length >> 8)])
        zlib.append(raw)
        var a: UInt32 = 1
        var b: UInt32 = 0
        for byte in raw {
            a = (a + UInt32(byte)) % 65521
            b = (b + a) % 65521
        }
        zlib.append(bigEndian: (b << 16) | a)

        var ihdr = Data()
        ihdr.append(bigEndian: side)
        ihdr.append(bigEndian: side)
        ihdr.append(contentsOf: [8, 2, 0, 0, 0])  // 8bit・RGB
        var png = Data([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])
        png.appendChunk("IHDR", ihdr)
        png.appendChunk("IDAT", zlib)
        png.appendChunk("IEND", Data())
        return URL(string: "data:image/png;base64,\(png.base64EncodedString())")
    }()
}

/// 表示判定の純粋関数(View から切り出す)。
public enum SpeciesImageDisplay {
    public enum LoadPhase: Equatable, Sendable {
        case loading
        case success
        case failure
    }

    /// 画像を出すか(true)、エンブレムのままか(false)。success のときだけ画像。
    public static func showsImage(phase: LoadPhase) -> Bool {
        phase == .success
    }

    /// 問い合わせるキー。nil・空・前後空白だけは問い合わせない(nil = エンブレム)。
    public static func lookupKey(speciesKey: String?) -> String? {
        guard let speciesKey, !speciesKey.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
        return speciesKey
    }
}

private extension Data {
    mutating func append(bigEndian value: UInt32) {
        append(contentsOf: [UInt8(value >> 24), UInt8((value >> 16) & 0xFF), UInt8((value >> 8) & 0xFF), UInt8(value & 0xFF)])
    }

    mutating func appendChunk(_ type: String, _ body: Data) {
        append(bigEndian: UInt32(body.count))
        var typed = Data(type.utf8)
        typed.append(body)
        append(typed)
        append(bigEndian: typed.crc32)
    }

    var crc32: UInt32 {
        var crc: UInt32 = 0xFFFF_FFFF
        for byte in self {
            crc ^= UInt32(byte)
            for _ in 0..<8 { crc = (crc & 1) != 0 ? (crc >> 1) ^ 0xEDB8_8320 : crc >> 1 }
        }
        return ~crc
    }
}
