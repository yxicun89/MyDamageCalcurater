# タイプバランスチェッカー設計レビュー文書

更新日: 2026-09-21
目的: Claude Code に設計レビューを依頼し、Codex がこの設計書を実装の唯一の起点として作業できる状態にする。

---

## 1. 今回の役割分担

AIエージェント間で実装対象を明確に分離する。

- **Claude Code**: 既存のダメージ計算アプリ `pokecalc-kit-v2` 専任
- **Codex**: 新しいタイプバランスチェッカー専任
- **共有対象**: 設計判断、API契約、共通データ仕様、作業ログのみ
- **共有しないもの**: 片方の未完了実装タスクを、もう片方へ「引き継ぎ作業」として渡さない

Codexには「Claudeの作業を引き継ぐ」のではなく、**この設計書を読んでタイプバランスチェッカーを新規実装する**よう指示する。

Claudeには、タイプバランスチェッカーのコード実装を依頼せず、ダメージ計算アプリとの整合性・共通化可能部分・設計上の問題点だけをレビューしてもらう。

---

## 2. プロジェクトの目的

既存のポケモンダメージ計算アプリとは別の責務として、自分用の**タイプバランスチェッカー**を構築する。

主目的は以下。

1. 最大6体のパーティについて、防御面のタイプ相性を一覧化する。
2. 将来的に攻撃範囲、特性による相性変化、仮想敵診断まで拡張する。
3. ダメージ計算アプリと同じ Kubernetes クラスタ上で、独立したサービスとして動作させる。
4. 1つのクラスタ内で複数サービスを分離して運用し、Deployment / Service / Ingress / HPA など Kubernetes の実践経験を増やす。
5. Argo CD を利用した GitOps を導入し、Git 上の宣言的な変更をクラスタへ自動反映できるデプロイフローを構築する。

このアプリは個人利用を前提とし、過剰な分散構成や独立マイクロサービス群は作らない。

---

## 3. 設計方針

### 3.1 アーキテクチャ

ダメージ計算アプリとタイプバランスチェッカーを、**同一 Kubernetes クラスタ上の別サービス**として動かす。

```text
Kubernetes Cluster
├─ Ingress
│  ├─ /api/damage  -> damage-service
│  └─ /api/balance -> balance-service
│
├─ damage-calc
│  ├─ Deployment
│  ├─ Service
│  └─ Pods
│
└─ type-balance
   ├─ Deployment
   ├─ Service
   └─ Pods
```

重要なのは、**1つのアプリに統合するのではなく、1つのクラスタ上で別Deployment / 別Serviceとして分離すること**。

これにより、例えば以下のようにワークロードごとに独立して設定できる。

```text
damage-calc
- replicas: 複数
- HPA対象
- 将来的に高負荷試験

type-balance
- replicas: 1から開始
- 軽量API
- 必要になった時だけHPA追加
```

ローカル開発では Docker Desktop Kubernetes / kind / minikube のいずれかを利用する。

### 3.2 Kubernetesを採用する理由

今回サーバレスではなく Kubernetes に統一する理由は次の通り。

- 既に API Gateway / Lambda / SAM を利用した経験がある
- 今回は未経験・学習余地の大きい Kubernetes に時間を使いたい
- 1クラスタ内で複数サービスを分離して動かす経験を得られる
- Deployment / Service / Ingress / HPA / Rolling Update / Observability を実践できる
- ダメージ計算とタイプバランスで異なる負荷特性を比較できる
- 将来的に Redis、監視、負荷試験などの共通基盤を追加しやすい

したがって、**両アプリを1つのKubernetesクラスタで運用しつつ、サービス単位では分離する**。

---

## 4. 技術スタック案

既存ダメージ計算アプリと可能な範囲で言語・型・データ定義を合わせる。

### Frontend

- React
- TypeScript
- 既存UIコンポーネントを再利用可能なら再利用

### Backend

- Go を第一候補とする
- HTTP API
- Docker
- Kubernetes Deployment
- Kubernetes Service
- Ingress
- Argo CD（GitOpsによるデプロイ）

### Local Kubernetes

候補:

- Docker Desktop Kubernetes
- kind
- minikube

既存のダメージ計算アプリで採用している方式がある場合は、それに統一する。

### Production候補

クラウドへデプロイする場合も、特定クラウドに依存しないKubernetes標準リソースを基本とする。

候補:

- AWS: Amazon EKS
- GCP: Google Kubernetes Engine (GKE)

原則として Deployment / Service / ConfigMap / Secret / HPA / Ingress など、AWS・GCPのどちらでも扱えるKubernetes標準リソースを中心に設計する。

クラウド固有機能は、実際にデプロイ先を決めた段階で追加する。例えば以下。

- AWS: ALB Ingress Controller / AWS Load Balancer Controller, ECR, CloudWatch など
- GCP: GKE Ingress / Gateway, Artifact Registry, Cloud Logging / Monitoring など

現時点では AWS / GCP のどちらかに固定せず、**まずローカルKubernetes上で両サービスを安定して動かす**。

### GitOps / Argo CD

デプロイは **Argo CD を利用した GitOps** を基本方針とする。

アプリケーションのデプロイ時に `kubectl apply` を手作業で繰り返すのではなく、Git 上の Kubernetes 定義を正本として扱い、Argo CD がクラスタの状態を同期する。

```text
Developer / Claude / Codex
        ↓
Git commit / push / merge
        ↓
Git Repository
  ├─ app source
  └─ k8s manifests / Helm / Kustomize
        ↓
Argo CD
        ↓
Kubernetes Cluster
  ├─ damage Deployment / Service
  └─ balance Deployment / Service
```

目標は、通常のデプロイでは **Git 上のイメージタグやmanifestを変更してマージすれば、Argo CD が差分を検知して反映できる状態** にすること。

Argo CD は Kubernetes 上で動作するため、AWS EKS / GCP GKE のどちらを選んでも同じ運用思想を維持する。

方針:

- Git を desired state の正本とする。
- Argo CD Application で対象ディレクトリを監視する。
- 初期構築・障害対応を除き、手動 `kubectl apply` を通常デプロイ手段にしない。
- 自動同期を採用する場合は `prune` / `selfHeal` の設定を理解した上で有効化する。
- アプリケーションコードとKubernetes設定の変更履歴をGitで追跡可能にする。
- Claude/Codexが共有基盤を変更する場合も、可能な限りGit上の宣言を変更しArgo CD経由で反映する。

初期段階では plain YAML / Kustomize / Helm のいずれを採用するかは既存構成を確認して決定する。過剰なテンプレート化は避ける。

### Persistence

TB1の初期実装では専用DBは不要。

将来的に以下が必要になった場合のみDBを追加する。

- パーティ保存
- お気に入り
- 履歴
- ユーザー設定

---

## 5. 計算コアの設計

計算ロジックはHTTP / Kubernetesの実行環境から分離する。

推奨構成:

```text
backend/
  cmd/
    api/
      main.go
  internal/
    balance/
      defense.go
      offense.go
      ability.go
      model.go
  adapter/
    http/
    master/
```

重要事項:

- `balance` パッケージは純粋Goとして実装する。
- HTTP handler 内にタイプ計算ロジックを書かない。
- Kubernetes固有設定はmanifest / deployment層に閉じ込める。
- 単体テストではKubernetesを起動せず計算コアを直接検証できるようにする。

将来的にダメージ計算アプリと共通ライブラリ化できる可能性は残すが、**最初から両プロジェクトを強結合しない**。

---

## 6. 段階実装

### TB0: 基盤

- ディレクトリ構成
- 型定義
- タイプ相性表
- Dockerfile
- Deployment / Service manifest
- Ingressルーティング
- Kubernetes上での最小疎通
- Argo CD Application定義またはGitOps対象ディレクトリの準備
- Git変更 → Argo CD同期 → Pod更新までの最小デプロイ確認
- 単体テスト基盤

### TB1: 防御タイプバランス

最大6体について18タイプそれぞれの防御倍率を計算する。

表示対象:

- 4倍弱点
- 2倍弱点
- 等倍
- 1/2耐性
- 1/4耐性
- 無効

チーム全体では、各攻撃タイプについて以下を集計する。

- 弱点持ち数
- 4倍弱点持ち数
- 耐性持ち数
- 無効持ち数
- 等倍数

**総合点・ランキング・独自スコアはTB1では作らない。**

### TB2: 攻撃範囲

選択されたポケモン/技から、攻撃面でどのタイプへ有効打を持つかを可視化する。

方針:

- 変化技は除外
- 同じ攻撃タイプの技を複数持っていても重複計上しない
- 防御タイプ分析とは別のデータ構造として扱う

### TB3: 特性

タイプ相性に影響する特性を反映する。

例:

- 特定タイプ無効
- 吸収
- 倍率変更

実装時に巨大な `abilityID` switch を作るのではなく、正規化された効果データを介して計算する。

タイプ由来の無効と、特性由来の無効は内部表現上区別する。

### TB4: 仮想敵診断

仮想敵のタイプや技範囲を入力し、現在のパーティで受けやすい/受けにくい箇所を表示する。

詳細仕様は TB1〜TB3 完成後に確定した(2026-09-22。ADR-0400: 仮想敵をポケモン + 技 ID で入力し、受ける最大倍率・与える最大倍率と安全に受けられる人数を返す)。

### TB5: おすすめタイプと該当ポケモン

チームの穴(防御・攻撃範囲)をふさげるタイプの候補と、そのタイプを持つ使用可能なポケモン全員を出す(2026-09-22 ユーザー要望。DECISIONS.md)。


### TB6: 技範囲チェッカー

技 ID(最大4つ)だけを入力し、変化技を除いた攻撃範囲(18単防御タイプの一貫判定)と、その技構成を半減以下(×1/2以下)で受けられる実在ポケモンを
図鑑から具体名で列挙する。特性で半減以下になるポケモンは別枠(2026-09-22 ユーザー要望。ADR-0404)。

---

## 7. 倍率表現

浮動小数点比較に依存しない。

候補:

```go
type Multiplier int

const (
    MultiplierZero    Multiplier = 0
    MultiplierQuarter Multiplier = 1
    MultiplierHalf    Multiplier = 2
    MultiplierNormal  Multiplier = 4
    MultiplierDouble  Multiplier = 8
    MultiplierQuad    Multiplier = 16
)
```

基準値4を1倍として整数で扱うなど、比較と集計が安定する表現を採用する。

APIレスポンスでは表示用文字列または分子/分母へ変換してよい。

---

## 8. API案

TB1の初期APIは小さく保つ。

### POST `/v1/team-balance/analyze`

Request例:

```json
{
  "members": [
    {
      "pokemonId": 1,
      "types": ["grass", "poison"]
    }
  ]
}
```

Response概念:

```json
{
  "members": [
    {
      "pokemonId": 1,
      "defense": {
        "fire": "2",
        "water": "0.5"
      }
    }
  ],
  "teamSummary": {
    "fire": {
      "weak": 1,
      "resist": 0,
      "immune": 0,
      "neutral": 0
    }
  }
}
```

ただし、ポケモンIDだけを送ってAPI側がタイプ情報を引くのか、フロント側で取得済みタイプを送るのかは設計レビュー対象とする。

判断基準:

- API責務
- マスタデータの一貫性
- 外部APIへの依存
- レートリミット
- キャッシュ
- オフライン性

---

## 9. マスタデータ / 外部API / レートリミット

タイプ相性計算のたびに外部サイトや外部APIへアクセスする構成にはしない。

基本方針:

1. ポケモン・タイプ相性などのマスタをローカル/自前側に保持する。
2. 更新が必要な場合だけ明示的に同期する。
3. ユーザー操作1回につき外部APIへ多数のリクエストを発生させない。
4. 取得元の利用規約・レート制限を尊重する。

タイプ相性表はアプリ内のバージョン管理された静的データでもよい。

既存ダメージ計算アプリのマスタデータ形式を安全に再利用できる場合は、形式や生成処理の共通化を検討する。ただし実行時にダメージ計算アプリのサービスへ強依存させない。

---

## 10. UI案

主画面はパーティ構築UIを中心とする。

表示候補:

1. 最大6体のメンバーカード
2. 18タイプの防御相性表
3. チーム集計
4. 注意カード
5. 将来: 攻撃範囲タブ

アクセシビリティ上、倍率を色だけで表現しない。

例:

- `×4 弱点`
- `×2 弱点`
- `×1 等倍`
- `×1/2 耐性`
- `×1/4 耐性`
- `×0 無効`

---

## 11. ダメージ計算アプリとの境界

### ダメージ計算アプリ（Claude担当）

- ダメージ計算API
- Docker / Kubernetes manifests
- 既存 `pokecalc-kit-v2` の改善
- Claude Codeが実装を継続

### タイプバランスチェッカー（Codex担当）

- タイプバランス分析API
- Docker / Kubernetes manifests
- Codexが実装

### 共通Kubernetes基盤

- 1つのクラスタを共有する
- Deploymentは分離する
- Serviceは分離する
- Ingressでルーティングする
- 必要に応じてNamespaceを共有する（例: `pokecalc`）
- 片方だけ独立してスケール可能にする
- Argo CDを共通のGitOpsデプロイ基盤として利用する
- Git上のmanifest / Helm / Kustomize定義をクラスタ状態の正本とする

### 共通化してよいもの

- Pokemon ID
- Type ID / Type name
- タイプ相性表の定義
- APIの命名規則
- データ生成スクリプト
- Kubernetesの命名規則
- Ingress / Gateway のルーティングルール
- クラウド非依存のKubernetes manifest方針
- Argo CD Application / GitOpsディレクトリ構成
- Goの純粋ドメインモデル（必要性が確認できた後）

### 共通化しないもの

- Deployment本体
- Service本体
- 各APIのビジネスロジック
- 片方の作業タスクをもう片方へ引き継ぐこと

---

## 12. Claudeにレビューしてほしい点

Claude Codeはこの文書を読んだうえで、**実装せず設計レビューのみ**行う。

特に以下を確認する。

1. 現在の `pokecalc-kit-v2` から安全に共通化できる型/データは何か。
2. 逆に共通化すると密結合になる部分は何か。
3. Goの計算コアを別プロジェクトに持たせる設計で問題がないか。
4. 同一Kubernetesクラスタで別Deployment / Serviceとして動かす構成に問題がないか。
5. マスタデータをどちらで保持するのが最も自然か。
6. 外部APIのレートリミットを避けるためのデータ配置が適切か。
7. AWS EKS / GCP GKE のどちらにも載せやすいクラウド非依存設計になっているか。
8. Ingress / LoadBalancer / Storage / Observability に不要なクラウド固有依存が入り込んでいないか。
9. TB1〜TB4の段階分割に依存関係上の問題がないか。
10. 既存ダメージ計算側を壊さず将来共通化できる境界になっているか。
11. Argo CDを使ったGitOps構成がAWS/GCPどちらでも維持できるか。
12. Argo CDのApplication分割を damage / balance で分けるべきか、共通Applicationにするべきか。
13. manifest管理に plain YAML / Kustomize / Helm のどれを使うのが現在の規模に適切か。

レビュー結果は `docs/ai-shared/claude-review.md` などの共有メモへ記録し、Codexが読める形にする。

---

## 13. AI間の作業共有ルール

AI同士が直接記憶を共有している前提にはしない。

**Git管理された共有ファイルを唯一の共有記憶として扱う。**

推奨:

```text
docs/
  ai-shared/
    CURRENT_STATE.md
    DECISIONS.md
    CLAUDE_LOG.md
    CODEX_LOG.md
```

### `CURRENT_STATE.md`

プロジェクト全体の現在地だけを短く記載する。

```md
# Current State

## Damage Calculator
Owner: Claude
Branch: ...
Status: ...
Next: ...

## Type Balance Checker
Owner: Codex
Branch: ...
Status: ...
Next: ...

## Shared Interfaces
- Pokemon ID: ...
- Type representation: ...
```

### `DECISIONS.md`

後から変えると影響が大きい判断だけを残す。

```md
## 2026-09-21: Damage Calculator / Type Balance both use Kubernetes
Decision:
- 1つのKubernetesクラスタ上で別Deployment / Serviceとして運用
- IngressまたはGatewayで `/api/damage` と `/api/balance` を分離
- AWS / GCPのどちらにもデプロイできるクラウド非依存設計を基本とする

Reason:
- Lambda / API Gateway / SAMは既に経験済み
- Kubernetesの実践経験を優先
- 複数サービス・独立スケーリング・運用を学ぶため

Impact:
- Type BalanceもDocker化する
- 共通クラスタ仕様はClaude/Codex間で共有
- 各サービスの実装責任は分離する
```

```md
## 2026-09-21: Argo CDを共通デプロイ基盤として採用
Decision:
- GitOpsを採用し、Git上のKubernetes定義をdesired stateの正本とする
- Argo CDでクラスタへ同期する
- AWS/GCPのどちらを選んでも同じデプロイ思想を維持する

Reason:
- 手動デプロイを減らし、変更履歴と実クラスタ状態を一致させたい
- Argo CDを使ったKubernetes運用を実践したい

Impact:
- Kubernetes設定変更は原則Git経由で行う
- damage / balanceの両サービスをArgo CD管理対象にする
```

### `CLAUDE_LOG.md` / `CODEX_LOG.md`

各セッション終了時に追記する。

```md
## YYYY-MM-DD HH:mm

### Done
- ...

### Changed files
- ...

### Decisions
- ...

### Open issues
- ...

### Next
- ...
```

重要:

- ログには会話全文を書かない。
- 実装済み事実・決定・未解決事項だけを書く。
- 相手AIはまず `CURRENT_STATE.md` と `DECISIONS.md` を読む。
- 必要な場合のみ相手のログを読む。
- Claude/Codexは相手の担当コードを勝手に変更しない。

---

## 14. Git / GitOps運用

担当ごとにブランチを分ける。

例:

```text
main
├─ feat/damage-calc-...      # Claude
└─ feat/type-balance-...     # Codex
```

共通仕様変更が必要な場合:

1. `DECISIONS.md` に提案を書く。
2. 共通インターフェースへの影響を確認する。
3. 担当側ブランチで変更する。
4. もう一方は共有ファイルを読んで追従する。

「相手AIの作業を引き継ぐ」ことを通常運用にしない。

### Argo CDとの連携

通常のデプロイフローは以下とする。

```text
feature branch
   ↓
commit / test
   ↓
PR / merge
   ↓
Git上のKubernetes desired state更新
   ↓
Argo CD sync
   ↓
Kubernetes rollout
```

イメージビルドとレジストリpushはCIで自動化することを将来目標とする。

クラウド決定後のコンテナレジストリ候補:

- AWS: ECR
- GCP: Artifact Registry

ただしArgo CD自体の運用は特定クラウドに依存させない。

---

## 15. Codex実装開始条件

Codexは次回レートリミット回復後、以下の順番で開始する。

1. この設計書を読む。
2. `CURRENT_STATE.md` / `DECISIONS.md` があれば読む。
3. Claudeのダメージ計算作業を引き継がない。
4. タイプバランスチェッカー専用ブランチを確認/作成する。
5. TB0から実装する。
6. 既存コードとの共通化は、必要性を確認してから最小限行う。
7. Kubernetes manifest / GitOps定義を変更した場合はArgo CDで同期可能な形になっていることを確認する。
8. セッション終了前に `CODEX_LOG.md` と `CURRENT_STATE.md` を更新する。

---

## 16. Claudeレビュー後に確定したい未決事項

- タイプバランスチェッカーを同一リポジトリ内に置くか、別リポジトリにするか
- Pokemonマスタの正本をどこに置くか
- API requestで `pokemonId` のみ送るか、型情報まで送るか
- 共通Goモジュールを作る時期
- 永続化が必要になった場合にどのDBを採用するか
- plain YAML / Kustomize / Helm のどれを採用するか
- Argo CD Applicationをサービス単位で分けるか
- Argo CDの自動Sync / prune / selfHealをどこまで有効化するか
- 本番クラウドをAWS EKS / GCP GKEのどちらにするか
- 本番デプロイをTB1直後に行うか、ローカル完成後に行うか

この未決事項はClaudeレビューで意見を出してもらうが、Claudeはタイプバランスチェッカーの実装を開始しない。
