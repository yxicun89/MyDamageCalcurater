// ItemRoles: 持ち物を役割で絞る・メガ種族の持ち物をメガストーンに固定する・持ち物の表示名(ADR-0509)。
//
// spec-writer のスタブ。シグネチャだけを決めてあり、中身は implementer が ADR-0509 のとおりに書く
// (いまは「従来の挙動」を返すので、ItemRoleFilterTests・MegaItemLockTests が期待どおりの理由で失敗する)。

/// 持ち物の欄が求める役割(ADR-0509 §2・§3)。
public enum ItemRoleRequirement: Equatable, Sendable {
    /// 攻撃側の欄(`roles` に attacker を含む持ち物)。
    case attacker
    /// 防御側の欄(`roles` に defender を含む持ち物)。
    case defender
    /// 攻守が決まらない欄(どちらかの役割を持つ持ち物。構築・判定・調整)。
    case any
}

/// 役割で絞る唯一の関数(ADR-0509 §2)。全画面で共有する。
public enum ItemRoleFilter {
    /// `items` のうち `requirement` に合う持ち物をマスタの順で返す。
    /// - `roles == nil`(不明)の持ち物は絞らずに出す。`roles` が空の持ち物は出さない。
    /// - `isMegaStone == true` の持ち物は常に出さない。
    /// - `keeping`(いまの選択)は、規則で外れても `items` にあればその1件だけ残す(§5)。
    public static func options(_ items: [Item], for requirement: ItemRoleRequirement, keeping selectedId: String? = nil) -> [Item] {
        // TODO(implementer): ADR-0509 §2 の表どおりに絞る。
        items
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
        // TODO(implementer)
        .none
    }

    /// `MegaSpeciesInfo` 版(VM が覚えている情報から作る)。
    public static func make(for info: MegaSpeciesInfo?, allItems: [Item]) -> MegaItemLock {
        // TODO(implementer)
        .none
    }

    /// 固定中なら要求に載せる持ち物 ID(`.locked` はストーン、`.missing` は nil)。`.none` は nil。
    public var lockedItemId: String? {
        // TODO(implementer)
        nil
    }

    /// 持ち物欄を操作できないか(`.locked` と `.missing`)。
    public var disablesItemField: Bool {
        // TODO(implementer)
        false
    }

    /// 種族を変えたときの持ち物 ID(ADR-0509 §4)。
    public static func itemIdAfterSpeciesChange(previous: MegaItemLock, next: MegaItemLock, currentItemId: String?) -> String? {
        // TODO(implementer)
        currentItemId
    }

    /// 構築の保存データの補正(ADR-0509 §4。Web ADR-0320 PR-B と同じ方針)。
    public static func correction(currentItemId: String?, lock: MegaItemLock) -> MegaItemCorrection {
        // TODO(implementer)
        .unchanged
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

/// 持ち物の表示名の唯一の関数(ADR-0509 §6)。メガストーンの `nameJa` は画面に出さない。
public enum ItemDisplayName {
    /// nil →「持ち物なし」/ `megaStoneNames` にある → その名前 / `isMegaStone == true` →「メガストーン」/
    /// それ以外 → `nameJa`(マスタに無い ID は ID のまま)。
    public static func text(itemId: String?, items: [Item], megaStoneNames: [String: String] = [:]) -> String {
        // TODO(implementer)
        BulkRowDisplay.itemLabel(itemId: itemId, items: items)
    }
}

/// メガの持ち物固定の文言(Web `web/src/i18n/ja.ts` の `megaItemText` と同じ語。ADR-0509 §7)。
public enum MegaItemText {
    /// 「{基本種名}のメガストーン」。基本種名が nil なら「メガストーン」だけ(名前を推測しない)。
    public static func stoneName(baseSpeciesNameJa: String?) -> String {
        // TODO(implementer)
        ""
    }

    // TODO(implementer): 以下は Web と同じ語にする(ADR-0509 §7 の表)。
    public static let lockedReason = ""
    public static let missingReason = ""
    public static let compareDisabledReason = ""
    public static func fixedItemName(_ name: String) -> String { "" }
    public static func correctedNotice(_ itemName: String) -> String { "" }
    public static let clearedNotice = ""
}
