/// 下部タブ(ナビゲーション案 B。ADR-0531)。関連する画面をまとめた区画で、宣言順がタブの並び。
/// どの画面がどのタブに入るかは各画面の登録(`FeatureSpec.tab`)が決める(ここに画面の一覧を書かない)。
public enum AppTab: String, CaseIterable, Hashable, Sendable {
    /// 計算・逆算・調整(タブの上部の区切りで切り替える)。
    case damage
    /// 構築一覧 → 編集。
    case team
    /// 素早さの表。
    case speed
    /// お気に入り・計算履歴。
    case records

    /// タブのラベル(短く。VoiceOver でもこの文言を読む)。
    public var title: String {
        switch self {
        case .damage: AppNavigationText.damageTab
        case .team: AppNavigationText.teamTab
        case .speed: AppNavigationText.speedTab
        case .records: AppNavigationText.recordsTab
        }
    }

    /// タブバーのボタンの識別子(UI テスト用。画面の入れ替えで変わらないよう、タブそのものに付ける)。
    public var accessibilityIdentifier: String { "tab-\(rawValue)" }

    /// タブ内の区切り(2 つ以上の画面があるとき)の入れ物の識別子。
    public var segmentsAccessibilityIdentifier: String { "tabSegments-\(rawValue)" }
}

/// タブ・区切りの選択と、訪れた画面(ADR-0531)。View に依存しない値の型で、ルートの View が
/// 1 つだけ持つ。画面(とその入力)はタブや区切りを移っても破棄しない — View は `visitedFeatureIDs` の画面だけを
/// 作り、作ったものは残す(Web の ADR-0308 と同じ「訪れた画面は保持」)。
public struct AppNavigationModel: Equatable, Sendable {
    /// 画面のあるタブ(`AppTab` の宣言順)。画面の無いタブは出さない。
    public let tabs: [AppTab]
    /// タブごとの区切り(画面の ID。`order` 順)。
    private let segmentsByTab: [AppTab: [String]]
    /// 選択中のタブ。
    public private(set) var selectedTab: AppTab
    /// タブごとの、最後に選んだ区切り(タブを移って戻ると同じ区切りを出す)。
    private var selectedSegmentByTab: [AppTab: String]
    /// 一度でも表示した画面(View はこれだけを作って保持する)。
    public private(set) var visitedFeatureIDs: Set<String>

    /// `specs` のうち `tab` を持つ画面から組む。最初は先頭のタブの先頭の区切りを選ぶ。
    public init(specs: [FeatureSpec]) {
        let placed = specs.filter { $0.tab != nil }.sorted { $0.order < $1.order }
        var segments: [AppTab: [String]] = [:]
        for spec in placed {
            guard let tab = spec.tab else { continue }
            segments[tab, default: []].append(spec.id)
        }
        segmentsByTab = segments
        tabs = AppTab.allCases.filter { segments[$0] != nil }
        selectedSegmentByTab = segments.compactMapValues(\.first)
        selectedTab = tabs.first ?? .damage
        visitedFeatureIDs = []
        if let current = currentFeatureID { visitedFeatureIDs.insert(current) }
    }

    /// タブ内の区切り(画面の ID)。画面の無いタブは空。
    public func segments(in tab: AppTab) -> [String] {
        segmentsByTab[tab] ?? []
    }

    /// タブで選ばれている画面。画面の無いタブは `nil`。
    public func selectedFeatureID(in tab: AppTab) -> String? {
        selectedSegmentByTab[tab]
    }

    /// いま見えている画面。
    public var currentFeatureID: String? {
        selectedFeatureID(in: selectedTab)
    }

    /// 画面の入っているタブ。タブに属さない画面(右上のアイコン・画面の中から積む画面)は `nil`。
    public func tab(of featureID: String) -> AppTab? {
        tabs.first { segments(in: $0).contains(featureID) }
    }

    /// タブの中で訪れた画面(区切りの並び順)。
    public func visitedFeatureIDs(in tab: AppTab) -> [String] {
        segments(in: tab).filter(visitedFeatureIDs.contains)
    }

    /// タブを選ぶ。画面の無いタブは無視する。
    public mutating func select(tab: AppTab) {
        guard tabs.contains(tab) else { return }
        selectedTab = tab
        if let current = currentFeatureID { visitedFeatureIDs.insert(current) }
    }

    /// 画面を選ぶ(そのタブへ移り、区切りも切り替える)。タブに属さない画面は無視する。
    public mutating func select(featureID: String) {
        guard let tab = tab(of: featureID) else { return }
        selectedSegmentByTab[tab] = featureID
        selectedTab = tab
        visitedFeatureIDs.insert(featureID)
    }
}

/// ナビゲーションの文言(1 か所に持つ。coding-rules §2)。
public enum AppNavigationText {
    public static let damageTab = "ダメージ"
    public static let teamTab = "構築"
    public static let speedTab = "素早さ"
    public static let recordsTab = "記録"
    /// タブ内の区切りの VoiceOver の見出し。
    public static let segmentsLabel = "画面の切り替え"
}

/// 画面の持ち方(ADR-0531 §2)。画面を離れたときに進行中の入力・送信を取り消すかを決める。
public enum ScreenLifetime: Sendable {
    /// タブ・区切りの画面。破棄しないので、離れても(タブ・区切りを移っても)送信は最後まで完了させる。
    /// 取り消すと「入力は新しいのに結果は古い」「送信が黙って消える」になるため。
    case retained
    /// 画面の中から積んだ画面(記録から開いた計算など)。戻ると破棄されるので、離れたら取り消す(issue #113 A6)。
    case pushed

    /// `.onDisappear` で `cancelPendingWork()` を呼ぶか。
    public var cancelsPendingWorkOnDisappear: Bool {
        switch self {
        case .retained: false
        case .pushed: true
        }
    }
}
