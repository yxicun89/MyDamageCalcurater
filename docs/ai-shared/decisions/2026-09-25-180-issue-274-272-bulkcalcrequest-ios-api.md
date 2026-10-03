## 2026-09-25: issue #274/#272 の「防御側の詳細(ランク・特性・状態異常)」を BulkCalcRequest に足す提案を採用(iOS レーン → API レーン)
Decision: iOS レーンの提案(PR #377。DECISIONS.md 2026-09-25「issue #274」の「提案(API レーン)」)を、下記の形に少し具体化して受け入れる。
`BulkCalcRequest` に任意の `defenderOverride: { abilityId?: string, ranks?: RankBlock, status?: StatusCondition }` を追加する
(コンポーネントスキーマとして新設し、既存の `RankBlock`/`StatusCondition` をそのまま再利用する。`Individual` の同名フィールドと型を揃える)。
指定した値は生成するすべての行(全プリセット × 全 itemVariants)の防御側に一律で上書きする(プリセットが決める SP・性格・持ち物・種族には触れない)。
省略時は現状と完全に同じ(何も送らなければ要求は今までどおり)。
実装は calc-svc だけでなく **engine 側の変更を伴う**(`engine.BulkInput` にオーバーライドを足し、プリセットから作った防御側 `Individual` に
プリセット解決の後・ダメージ計算の前で当てる。`engine/` は純粋なので DB・HTTP 等の外部依存は増えない。CLAUDE.md 絶対ルール2 に抵触しない)。
engine を変更するため ADR-0003 の test-first(spec-writer が先にテストを書く)+ 独立 critic の適用対象。
Reason: iOS が計算画面の「詳細」で防御側のランク・特性・状態異常を選べるようにしたいが(issue #274・#272)、
`BulkCalcRequest` は `defenderSpeciesKey` しか持たず、防御側プリセット(ADR-0009)が決める SP・性格・持ち物以外の
上書き手段が契約に無い。名前を `defenderOverride`(anonymous object ではなく再利用可能なコンポーネントスキーマ)にする方が、
将来 `calcReverse` 等の別操作で同じ形が要るときに使い回しやすい。
Impact: **API レーンの実装は M2(P5-3 record-svc・P5-4 team-svc)の後に着手する**(iOS レーンからも「急ぎではない」と
明記されている。docs/plan.md「改善要望」に記録し、着手まで issue はクローズしない)。実装後は
`api/openapi.yaml` → `make gen`(Web・iOS 双方の生成物)→ calc-svc の `resolveIndividual` と同じ検証(種族に無い特性・
状態異常の enum)を防御側オーバーライドにも適用、の順で進める。入ったら iOS・Web が「詳細」に「防御側のランク(B/D)」
「防御側の特性」「防御側の状態異常」を足す(iOS・Web 側の作業。この決定では扱わない)。
