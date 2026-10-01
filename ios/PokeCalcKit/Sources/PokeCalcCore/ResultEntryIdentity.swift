// ResultEntryIdentity: 一括計算の行・逆算の候補の ID を、特性で分かれたときにも一意にする
// (issue #272。ADR-0501「P6-19」1章)。
//
// サーバーは防御側(逆算は相手)の特性を省略すると種族の特性をすべて試し、結果が違うときだけ
// 同じ(プリセット, 持ち物)・(性格クラス, 持ち物)の行・候補を特性ごとに分ける(ADR-0126・ADR-0214)。
// 画面の ID(`<preset>@<item>`・`<natureClass>@<item>`)は特性を含まないので、分かれた行が同じ ID になり、
// SwiftUI の `ForEach` と XCUITest の identifier(`calcResultRow-<id>` 等)が衝突する。
//
// 規則(既存の identifier をできるだけ変えない): 同じ base ID の行・候補が2つ以上あるときだけ、その行・候補の
// ID に `@<代表の特性 ID>` を足す。1つしかない base ID は今までどおり(特性で分かれない大半の技、
// 特性を指定したとき、特性の情報が無い行)。純粋な関数だけを持つ。

/// 行・候補の ID の組み立て。
public enum ResultEntryIdentity {
    /// base ID と特性の区切り(base ID の `<preset>@<item>` と同じ記号)。
    public static let abilitySeparator = "@"
    /// 特性の情報が無い(`abilityId == nil`)行が分かれたときに特性の代わりに入れる記号(持ち物なしの `-` と同じ)。
    public static let noAbilityPlaceholder = "-"
    /// 契約違反(同じ base・同じ特性の行が2つ以上)でも一意にするため、2つ目以降に足す通し番号の頭。
    public static let duplicateOrdinalPrefix = "#"

    /// `baseIDs[i]`(`<preset>@<item>` など)と `abilityIds[i]`(その行の代表の特性。nil は情報なし)から、
    /// 入力と同じ順・同じ件数の一意な ID を作る。
    ///
    /// 1. base ID が入力の中で1回だけ → base ID のまま。
    /// 2. 2回以上 → `<base>@<abilityId ?? "-">`。
    /// 3. それでも重なる ID(契約違反)は、2つ目以降に `#2`・`#3`…(入力の順)を足す。1つ目はそのまま。
    ///
    /// `baseIDs.count != abilityIds.count` は呼び出し側の誤り(`abilityIds` の足りない分は nil とみなす)。
    public static func uniqueIDs(baseIDs: [String], abilityIds: [String?]) -> [String] {
        let split = splitBaseIDs(baseIDs)
        // 規則1・2: 分かれた base だけに `@<代表の特性 ID>` を足す。
        let candidates: [String] = baseIDs.enumerated().map { index, base in
            guard split.contains(base) else { return base }
            let abilityId = index < abilityIds.count ? (abilityIds[index] ?? noAbilityPlaceholder) : noAbilityPlaceholder
            return "\(base)\(abilitySeparator)\(abilityId)"
        }
        // 規則3: それでも重なる ID は2つ目以降に通し番号を足す(入力の順)。
        var seenCounts: [String: Int] = [:]
        return candidates.map { id in
            let count = (seenCounts[id] ?? 0) + 1
            seenCounts[id] = count
            return count == 1 ? id : "\(id)\(duplicateOrdinalPrefix)\(count)"
        }
    }

    /// 入力の中で2回以上現れる base ID(= 特性で分かれた組)。副題(`AbilityGroupLabel`)を出す行を決めるのに使う。
    public static func splitBaseIDs(_ baseIDs: [String]) -> Set<String> {
        var counts: [String: Int] = [:]
        for base in baseIDs {
            counts[base, default: 0] += 1
        }
        return Set(counts.filter { $0.value > 1 }.map(\.key))
    }
}
