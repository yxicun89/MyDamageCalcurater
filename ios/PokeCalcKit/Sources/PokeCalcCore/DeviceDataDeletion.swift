import Observation

// DeviceDataDeletion: 「この端末のデータを削除」(P6-7・issue #103・ADR-0209 §5・§8・ADR-0501「P6-7」)。
//
// record と team は別サービス・別 DB なので、削除 API も2本(`DELETE /api/record/device-data`・
// `DELETE /api/team/device-data`)。画面は両方が completed になってから「削除しました。」を出す。
// 計算・逆算の `PokeCalcService` には混ぜない(絶対ルール5: 削除の失敗が計算に影響しない)。

/// 1回の削除要求の結果(openapi `DeletionStatus`)。
public enum DeletionProgress: Equatable, Sendable {
    /// この端末のデータは残っていない。
    case completed
    /// 1回の上限に達した。残りがあるので同じ要求をもう一度送る。
    case partial
}

/// 削除の対象サービス。
public enum DeviceDataTarget: CaseIterable, Sendable {
    case record
    case team
}

/// 端末単位の全削除の境界。`PokeCalcService` とは別のプロトコル(計算と無関係に保つ)。
/// 実装は `APIPokeCalcService`(2本の DELETE)と `MockDeviceDataService`。
/// 各メソッドは1回の HTTP 要求に対応し、冪等(何も無くても `.completed`)。失敗は `PokeCalcError`。
public protocol DeviceDataService: Sendable {
    func deleteRecordDeviceData() async throws -> DeletionProgress
    func deleteTeamDeviceData() async throws -> DeletionProgress
}

/// 画面の固定文言(ADR-0209 §8 の文言案を iOS 向けに確定したもの。ADR-0501「P6-7」2章)。
/// マスタには無い表示専用の文言なので `AboutText` と同じくここに1か所だけ持つ。
public enum DeviceDataText {
    /// 設定(説明)の3文。順序固定。
    public static let explanation: [String] = [
        "アカウントはありません。履歴・お気に入り・構築は、この端末に割り当てた ID でサーバーに保存しています。",
        "ID が変わると(アプリを削除して入れ直したとき)、前のデータは開けなくなります。元に戻す方法はありません。",
        "開けなくなったデータは自動的に消えます。計算の履歴は記録から90日、お気に入りと構築は最後に使った日から18か月です。",
    ]
    public static let deleteButton = "この端末のデータを削除"
    public static let confirmMessage = "履歴・お気に入り・構築をサーバーから削除します。元に戻せません。"
    public static let confirmAction = "削除する"
    public static let cancelAction = "キャンセル"
    public static let deleting = "削除しています…"
    public static let partialNotice = "まだ残っています。続けて削除します。"
    public static let failure = "サーバーに届きませんでした。通信を確認してもう一度お試しください。"
    /// セクションの見出し。
    public static let sectionTitle = "データの扱い"
    public static let completed = "削除しました。"
    public static let retryButton = "もう一度削除する"
    public static let recordLabel = "履歴・お気に入り"
    public static let teamLabel = "構築"

    /// 片方だけ削除済みのときに失敗文言へ添える一文(例「構築は削除済みです。」)。
    public static func partlyDeleted(label: String) -> String {
        "\(label)は削除済みです。"
    }
}

/// 1つの対象の結果。
public enum DeviceDataTargetOutcome: Equatable, Sendable {
    /// まだ要求していない(またはキャンセルで中断した)。
    case pending
    case completed
    /// 上限回数 `partial` が続いた(残りあり。失敗ではない。再試行できる)。
    case incomplete
    /// 要求が失敗した(`PokeCalcError.code` を運ぶ)。
    case failed(code: String)
}

/// 「この端末のデータを削除」の状態(`@MainActor`。他の ViewModel と同じ)。
@MainActor
@Observable
public final class DeviceDataDeletionViewModel {
    public enum Phase: Equatable, Sendable {
        case idle
        case confirming
        case deleting
        case finished
    }

    /// 1つの対象に送る要求の上限(`partial` の繰り返しで無限ループしない)。
    public static let defaultMaxRequestsPerTarget = 20

    public private(set) var phase: Phase = .idle
    public private(set) var recordOutcome: DeviceDataTargetOutcome = .pending
    public private(set) var teamOutcome: DeviceDataTargetOutcome = .pending

    private let service: any DeviceDataService
    private let maxRequestsPerTarget: Int

    public init(
        service: any DeviceDataService,
        maxRequestsPerTarget: Int = DeviceDataDeletionViewModel.defaultMaxRequestsPerTarget
    ) {
        self.service = service
        self.maxRequestsPerTarget = maxRequestsPerTarget
    }

    /// 直前の応答が `partial` だった(残りがあるので続けて削除している)。
    private var lastResponseWasPartial = false

    /// 両方 `completed`。
    public var isAllCompleted: Bool { recordOutcome == .completed && teamOutcome == .completed }
    /// 終了していて、まだ `completed` でない対象がある(再試行できる)。
    public var canRetry: Bool { phase == .finished && !isAllCompleted }

    /// 画面に出す状態文言。`idle`/`confirming` は nil。
    public var statusMessage: String? {
        switch phase {
        case .idle, .confirming:
            return nil
        case .deleting:
            return lastResponseWasPartial ? DeviceDataText.partialNotice : DeviceDataText.deleting
        case .finished:
            return finishedMessage
        }
    }

    private var finishedMessage: String {
        if isAllCompleted { return DeviceDataText.completed }
        let outcomes: [(label: String, outcome: DeviceDataTargetOutcome)] = [
            (DeviceDataText.recordLabel, recordOutcome), (DeviceDataText.teamLabel, teamOutcome),
        ]
        let hasFailure = outcomes.contains { if case .failed = $0.outcome { return true } else { return false } }
        // 失敗なし(上限まで partial が続いただけ)は、失敗ではなく「続けて削除する」案内にする。
        guard hasFailure else { return DeviceDataText.partialNotice }
        let done = outcomes.filter { $0.outcome == .completed }.map { DeviceDataText.partlyDeleted(label: $0.label) }
        return ([DeviceDataText.failure] + done).joined(separator: "\n")
    }

    /// ボタン押下。`idle`/`finished` なら `confirming`(確認ダイアログを出す)にする。通信しない。
    public func requestDeletion() {
        guard phase == .idle || phase == .finished else { return }
        phase = .confirming
    }

    /// 確認ダイアログの取り消し。`confirming` なら `idle` に戻す。通信しない。
    public func cancelConfirmation() {
        guard phase == .confirming else { return }
        phase = .idle
    }

    /// 確認ダイアログの了承。`confirming` のときだけ動く(それ以外は何もしない)。
    public func confirmDeletion() async {
        guard phase == .confirming else { return }
        recordOutcome = .pending
        teamOutcome = .pending
        await run(targets: DeviceDataTarget.allCases)
    }

    /// `finished` かつ `canRetry` のときだけ、`completed` でない対象だけを再度削除する(再確認は不要)。
    public func retry() async {
        guard canRetry else { return }
        await run(targets: DeviceDataTarget.allCases.filter { outcome(of: $0) != .completed })
    }

    // MARK: - 実行

    /// 対象を順に削除する。対象ごとに独立(片方が失敗・未完了でも次へ進む)。キャンセルされたら
    /// 以後の要求を送らず、失敗にもせず `idle` に戻す。
    private func run(targets: [DeviceDataTarget]) async {
        phase = .deleting
        lastResponseWasPartial = false
        for target in targets {
            if Task.isCancelled { return cancelRun() }
            guard let result = await delete(target) else { return cancelRun() }
            set(result, for: target)
        }
        phase = .finished
    }

    private func cancelRun() {
        lastResponseWasPartial = false
        phase = .idle
    }

    /// 1つの対象を `completed` まで(上限回数まで)繰り返す。キャンセルされたら nil。
    private func delete(_ target: DeviceDataTarget) async -> DeviceDataTargetOutcome? {
        for _ in 0..<maxRequestsPerTarget {
            do {
                let progress = try await request(target)
                if Task.isCancelled { return nil }
                switch progress {
                case .completed:
                    lastResponseWasPartial = false
                    return .completed
                case .partial:
                    lastResponseWasPartial = true
                }
            } catch is CancellationError {
                return nil
            } catch {
                lastResponseWasPartial = false
                if Task.isCancelled { return nil }
                let code = (error as? PokeCalcError)?.code ?? PokeCalcError.Code.transport
                return .failed(code: code)
            }
        }
        lastResponseWasPartial = false
        return .incomplete
    }

    private func request(_ target: DeviceDataTarget) async throws -> DeletionProgress {
        switch target {
        case .record: return try await service.deleteRecordDeviceData()
        case .team: return try await service.deleteTeamDeviceData()
        }
    }

    private func outcome(of target: DeviceDataTarget) -> DeviceDataTargetOutcome {
        switch target {
        case .record: return recordOutcome
        case .team: return teamOutcome
        }
    }

    private func set(_ outcome: DeviceDataTargetOutcome, for target: DeviceDataTarget) {
        switch target {
        case .record: recordOutcome = outcome
        case .team: teamOutcome = outcome
        }
    }
}
