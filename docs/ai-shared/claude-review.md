# タイプバランスチェッカー 設計レビュー(Claude)

日付: 2026-09-21 / レビュー対象: type_balance_architecture_review.md
結論: **この設計のまま実装開始してよい**。指摘は3点のみ。

## 承認
- 同一クラスタ・別Deployment/Serviceでの分離
- 計算コアを純粋Goパッケージにする構成(engine/ と同じ形)
- クラウド非依存のKustomize方針
- Argo CD Applicationをサービスごとに分割

## 修正指摘

### 1. マスタデータはAPI経由で取得する(DB直結禁止)
balance-svc は MySQL(pokedex 用)に直接接続しない。pokedex-svc の REST API を呼ぶ。
理由: DBスキーマ変更で balance-svc が壊れるのを防ぐため(CLAUDE.md 絶対ルール4と同じ考え方)。
→ 外部APIのレートリミット対策は不要になる(内部呼び出しのため)。

### 2. Goモジュールの共有はまだしない
型定義(PokemonID, TypeID, Multiplier)は今は各リポジトリ内で別々に定義してよい。
importで直結すると、一方の変更がもう一方のビルドを壊す。
再検討のタイミング: TB3着手時、または重複が本当に苦痛になったとき。

### 3. TB0でEffect構造を先に決めておく
TB3(特性)で「タイプ由来の無効」「特性由来の無効」を区別する設計になっているため、
TB1の戻り値の型にこの区別を最初から持たせておくこと。後からの型変更を避けるため。

```go
type EffectSource int
const (
    EffectSourceType EffectSource = iota
    EffectSourceAbility
)
type DefenseResult struct {
    Multiplier Multiplier
    Source     EffectSource // TB1時点では常に EffectSourceType
}
```

## 未決事項への回答
リポジトリ: 同一(pokecalc内 services/balance/) / マスタ正本: pokedex-svc(API経由)
/ API request: pokemonId のみ / 共通Goモジュール: 当面作らない / DB: 必要になるまで作らない
/ manifest: Kustomize / Argo CD: サービスごとに分割 / Sync: 最初は manual
/ 本番クラウド: 未決定のまま進行 / 本番デプロイ: ローカル安定後
