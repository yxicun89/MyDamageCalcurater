// ItemRoles: 持ち物を役割で絞る・メガ種族の持ち物をメガストーンに固定する・持ち物の表示名(ADR-0509)。

/// 持ち物の欄が求める役割(ADR-0509 §2・§3)。
public enum ItemRoleRequirement: Equatable, Sendable {
    /// 攻撃側の欄(`roles` に attacker を含む持ち物)。
    case attacker
    /// 防御側の欄(`roles` に defender を含む持ち物)。
    case defender
    /// 攻守の両方をする欄(どちらかの役割を持つ持ち物。判定・調整。Web の `either`)。
    case either
    /// 役割で絞らない欄(メガストーンだけ外す。構築。Web の `any`。ADR-0326 §2)。
    case any
}

/// 役割で絞る唯一の関数(ADR-0509 §2)。全画面で共有する。
public enum ItemRoleFilter {
    /// `items` のうち `requirement` に合う持ち物をマスタの順で返す。
    /// - `roles == nil`(不明)の持ち物は絞らずに出す。`roles` が空の持ち物は出さない。
    /// - `isMegaStone == true` の持ち物は常に出さない。
    /// - `keeping`(いまの選択)は、規則で外れても `items` にあればその1件だけ残す(§5)。
    public static func options(_ items: [Item], for requirement: ItemRoleRequirement, keeping selectedId: String? = nil) -> [Item] {
        items.filter { item in
            if item.isMegaStone == true { return false }
            if item.id == selectedId { return true }
            guard let roles = item.roles else { return true }
            switch requirement {
            case .attacker: return roles.contains(.attacker)
            case .defender: return roles.contains(.defender)
            case .either: return !roles.isEmpty
            case .any: return true
            }
        }
    }
}

/// メガ種族の情報(VM が `species(key:)` の応答ごとに覚える。ADR-0509 §4)。
public struct MegaSpeciesInfo: Equatable, Sendable {
    public var isMega: Bool
    public var requiredItemId: String?
    public var baseSpeciesNameJa: String?

    public init(isMega: Bool, requiredItemId: String?, baseSpeciesNameJa: String?) {
        self.isMega = isMega
        self.requiredItemId = requiredItemId
        self.baseSpeciesNameJa = baseSpeciesNameJa
    }

    public init(detail: SpeciesDetail) {
        self.init(isMega: detail.isMega, requiredItemId: detail.requiredItemId, baseSpeciesNameJa: detail.baseSpeciesNameJa)
    }
}

/// メガ種族の持ち物の固定(ADR-0509 §4。Web `megaItemLock` と同じ3状態)。
public enum MegaItemLock: Equatable, Sendable {
    /// メガではない(持ち物は自由に選べる)。
    case none
    /// メガ。持ち物は `itemId` に固定し、`displayName`(「{基本種名}のメガストーン」)を見せる。
    case locked(itemId: String, displayName: String)
    /// メガだが、ストーンをマスタの持ち物から引けない(持ち物は空・欄は操作不可)。
    case missing

    /// `detail` が nil・非メガなら `.none`。メガで `requiredItemId` が `allItems`(絞り込む前の全件)にあれば `.locked`、
    /// 無ければ `.missing`。
    public static func make(for detail: SpeciesDetail?, allItems: [Item]) -> MegaItemLock {
        make(for: detail.map(MegaSpeciesInfo.init(detail:)), allItems: allItems)
    }

    /// `MegaSpeciesInfo` 版(VM が覚えている情報から作る)。
    public static func make(for info: MegaSpeciesInfo?, allItems: [Item]) -> MegaItemLock {
        guard let info, info.isMega else { return .none }
        guard let stoneId = info.requiredItemId, let stone = allItems.first(where: { $0.id == stoneId }) else { return .missing }
        return .locked(itemId: stoneId, displayName: ItemDisplayName.megaStoneName(for: stone, baseSpeciesNameJa: info.baseSpeciesNameJa))
    }

    /// 固定中なら要求に載せる持ち物 ID(`.locked` はストーン、`.missing` は nil)。`.none` は nil。
    public var lockedItemId: String? {
        if case .locked(let itemId, _) = self { return itemId }
        return nil
    }

    /// 持ち物欄を操作できないか(`.locked` と `.missing`)。
    public var disablesItemField: Bool {
        self != .none
    }

    /// 種族を変えたときの持ち物 ID(ADR-0509 §4)。
    public static func itemIdAfterSpeciesChange(previous: MegaItemLock, next: MegaItemLock, currentItemId: String?) -> String? {
        switch (previous, next) {
        case (_, .locked(let itemId, _)): return itemId
        case (_, .missing): return nil
        case (.none, .none): return currentItemId
        case (_, .none): return nil
        }
    }

    /// 構築の保存データの補正(ADR-0509 §4。Web ADR-0320 PR-B と同じ方針)。
    public static func correction(currentItemId: String?, lock: MegaItemLock) -> MegaItemCorrection {
        switch lock {
        case .none: return .unchanged
        case .locked(let itemId, let displayName):
            return currentItemId == itemId ? .unchanged : .fixed(itemId: itemId, displayName: displayName)
        case .missing: return currentItemId == nil ? .unchanged : .cleared
        }
    }
}

/// 構築の保存データの補正の結果。
public enum MegaItemCorrection: Equatable, Sendable {
    case unchanged
    /// メガ種族の持ち物をストーンに直した(`displayName` は通知に使う)。
    case fixed(itemId: String, displayName: String)
    /// ストーンを引けないので持ち物を空にした。
    case cleared
}

/// 持ち物の表示名の唯一の関数(ADR-0509 §6。2026-10-04 に更新)。メガストーンは、マスタの `nameJa` が日本語の正式名称ならそのまま出し、
/// 英語名のフォールバックのときだけ「{基本種名}のメガストーン」を組み立てる(`megaStoneName`)。
public enum ItemDisplayName {
    /// nil →「持ち物なし」/ `megaStoneNames` にある → その名前 / `isMegaStone == true` → `megaStoneName(for:baseSpeciesNameJa: nil)`(正式名称、無ければ「メガストーン」)/
    /// それ以外 → `nameJa`(マスタに無い ID は ID のまま)。
    public static func text(itemId: String?, items: [Item], megaStoneNames: [String: String] = [:]) -> String {
        guard let itemId else { return noItemLabel }
        if let name = megaStoneNames[itemId] { return name }
        guard let item = items.first(where: { $0.id == itemId }) else { return itemId }
        return item.isMegaStone == true ? megaStoneName(for: item, baseSpeciesNameJa: nil) : item.nameJa
    }

    /// `text` に日本語の文字(ひらがな・カタカナ・漢字・長音「ー」)が1文字以上あるか(ADR-0509 追記 §6')。純粋関数。
    /// 英数字・全角英数字・記号・空文字だけなら false(マスタの英語名のフォールバックを見分ける)。
    public static func containsJapanese(_ text: String) -> Bool {
        text.unicodeScalars.contains { scalar in
            switch scalar.value {
            case 0x3041...0x30FF, 0x3400...0x4DBF, 0x4E00...0x9FFF, 0xFF66...0xFF9F: return true
            default: return false
            }
        }
    }

    /// メガストーンの表示名の唯一の関数(ADR-0509 追記 §6')。`item.nameJa` が日本語の文字を含む(正式名称)ならそのまま、
    /// 含まない(英語名のフォールバック)なら従来の `MegaItemText.stoneName(baseSpeciesNameJa:)`。
    /// `baseSpeciesNameJa` が nil でも正式名称ならそれを出す。
    public static func megaStoneName(for item: Item, baseSpeciesNameJa: String?) -> String {
        if containsJapanese(item.nameJa) { return item.nameJa }
        return MegaItemText.stoneName(baseSpeciesNameJa: baseSpeciesNameJa)
    }

    /// 「持ち物なし」の表示(`id` に持ち物が無いとき)。
    public static let noItemLabel = "持ち物なし"

    /// 名前を引く側(結果の行・未対応の印の注記など)に渡す、メガストーンの `nameJa` を日本語の表示名に置いた一覧
    /// (置いたものは `isMegaStone` を false にして、`nameJa` をそのまま表示名として使わせる)。
    public static func displayItems(_ items: [Item], megaStoneNames: [String: String] = [:]) -> [Item] {
        items.map { item in
            guard item.isMegaStone == true || megaStoneNames[item.id] != nil else { return item }
            var copy = item
            copy.nameJa = text(itemId: item.id, items: items, megaStoneNames: megaStoneNames)
            copy.isMegaStone = false
            return copy
        }
    }
}

/// メガの持ち物固定の文言(Web `web/src/i18n/ja.ts` の `megaItemText` と同じ語。ADR-0509 §7)。
public enum MegaItemText {
    /// 「{基本種名}のメガストーン」。基本種名が nil なら「メガストーン」だけ(名前を推測しない)。
    public static func stoneName(baseSpeciesNameJa: String?) -> String {
        guard let baseSpeciesNameJa else { return "メガストーン" }
        return "\(baseSpeciesNameJa)のメガストーン"
    }

    public static let lockedReason = "メガシンカ: メガストーンを持ちます"
    public static let missingReason = "メガシンカ: メガストーンがマスタに見つかりません"
    public static let compareDisabledReason = "メガシンカ: 防御側の持ち物はメガストーンに固定されるため、候補は比較しません"
    public static func fixedItemName(_ name: String) -> String { "持ち物: \(name)" }
    public static func correctedNotice(_ itemName: String) -> String {
        "メガシンカのため持ち物を\(itemName)に直しました。保存すると反映されます"
    }
    public static let clearedNotice = "メガシンカのメガストーンがマスタに無いため、持ち物を空にしました。保存すると反映されます"
}
