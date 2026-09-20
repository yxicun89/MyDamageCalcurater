# CLAUDE.md

自分用ポケモン ダメージ計算 & 構築ビルダー(ポケモンチャンピオンズ対応)。学習目的のマイクロサービス構成。

## 最初に読むもの(この順で)

1. `docs/plan.md` — 進行状況とタスク。**作業の起点は常にここ**
2. `docs/requirements.md` — 要件の正
3. `docs/test-strategy.md` — テストの正
4. `docs/design.md` — 画面・ビジュアルの正
5. `docs/adr/` — 過去の設計判断

## リポジトリ構成

```
api/openapi.yaml        # API契約の唯一の正(仕様先行)
engine/                 # 計算エンジン(純粋Go、I/O・外部依存なし)
engine/cmd/wasm/        # ブラウザ用WASMビルド
services/gateway/       # Echo。クライアントの唯一の入口、/assets も配信
services/pokedex/       # マスタ参照(MySQL)
services/calc/          # 計算(ステートレス)
services/record/        # 計算イベント・お気に入り(TiDB: record DB)
services/team/          # 構築(TiDB: team DB)
tools/importer/         # マスタ取込(k8s CronJob)
tools/golden/           # @smogon/calc からテストベクタ生成(Node)
tools/assets/           # 画像変換・アップロード
testdata/golden/        # 生成済みテストベクタ(コミットする)
web/                    # Vite + React + TypeScript
ios/                    # SwiftUI アプリ
deploy/k8s/             # Kustomize(base / overlays/local / overlays/cloud)
scripts/                # 環境チェック・補助スクリプト
docs/
```

## 絶対ルール

1. **API変更は必ず `api/openapi.yaml` から**。手書きでハンドラやクライアントの型を作らない。変更後は `make gen`
2. **`engine/` は純粋に保つ**。DB・HTTP・ファイル・時刻・乱数の外部取得を持ち込まない
3. **計算ロジックを変更したら `make test-golden` が全件一致すること**。既知の差分は `testdata/golden/known_diffs.yaml` にADR付きで登録したものだけ許容
4. **サービスは自分のDBにだけ触る**。他サービスのデータはAPIかイベント経由
5. **計算はイベント保存に依存しない**。record/team/TiDB/NATSが落ちても計算APIは成功を返す
6. **テストを消したり弱めたりして通さない**。期待値の変更は理由をコミットメッセージに書く
7. 設計判断をしたら `docs/adr/` に1ファイル追加する
8. 1タスク = 1コミット。`docs/plan.md` のチェックを更新してからコミット

## ドメイン規約

- 対象: ポケモンチャンピオンズ、シングル優先。engine の入力には最初から `format`(single/double)を持たせる
- Lv50、個体値31固定。能力ポイント(SP)は1ステータス最大32・合計66
  - HP = 種族値 + 75 + SP / その他 = floor((種族値 + 20 + SP) × 性格補正)
- ゴールデンテストでは SP を努力値 `max(0, 8×SP−4)` に換算して @smogon/calc(gen9)と照合
- ダメージ計算は4096基準の固定小数と五捨五超入。float で近似しない
- 逆算(調整推定)は engine 内の総当たり探索(WASMでも動かすため)
- 持ち物・技・ポケモンのリストをコードにハードコードしない。マスタデータから引く
- 画像は必須にしない。無ければタイプ色エンブレムで成立すること
- 常時動くアニメーションを入れない。演出は操作時のみ

## 技術規約

- Go: Echo、oapi-codegen(echo サーバー生成)、sqlc、golang-migrate。テーブル駆動テスト
- Web: Vite + React + TS、openapi-typescript、Playwright
- iOS: SwiftUI、swift-openapi-generator、XCTest。シミュレータで `xcodebuild test`
- 認証なし。クライアントは端末ID(UUID)とセッションIDを全リクエストに付与
- 設定は環境変数。ローカルとクラウドの差分は Kustomize overlay で吸収

## コマンド(Phase 0 で Makefile に実装する)

```
make doctor       # 前提ツールの確認(scripts/doctor.sh)
make up / down    # k3d クラスタ作成+全デプロイ / 削除
make dev          # k8s を使わずローカルで全サービス起動(高速な開発ループ用)
make gen          # OpenAPI / sqlc のコード生成
make test         # 全ユニットテスト(engine/services/web)
make test-golden  # engine のゴールデンテスト
make test-all-species # 全ポケモン網羅テスト
make e2e          # k3d 上のスモーク + Playwright
make ios-test     # iOS シミュレータでテスト
make wasm         # engine を WASM にビルドして web/public へ
make import       # マスタデータ取込
make assets       # 画像を WebP 2サイズに変換して MinIO へ
```

## 開発ワークフロー

タスクは `/phase` スキルで進める。各タスクで以下のサブエージェントを順に使う。

| ステップ | エージェント | モデル | 役割 |
|---|---|---|---|
| 1 | quick-scanner | haiku | 関連コード・ADR・要件を探索し影響範囲を要約 |
| 2 | spec-writer | opus | 受け入れ条件とテストを先に書く |
| 3 | implementer | sonnet | テストを通す最小実装 |
| 4 | critic | opus | ルール違反・テスト漏れ・越境をレビュー。NGなら3へ(最大3回) |
| 5 | Codex(任意) | GPT系 | engine・逆算・DB設計など重要タスクのみ `scripts/codex-review.sh` で別モデルレビュー |

- 同じ失敗で3回ループしたら止まり、`docs/plan.md` の「ブロッカー」に書いて次のタスクへ
- 完了条件: `make test` 成功 / engine 変更時は `make test-golden` 成功 / plan.md 更新 / 必要ならADR
- 改善要望は `/improve`、全体確認は `/verify`

## 人間の確認が必要なこと(自動で進めない)

- Xcode の署名チームの設定と実機インストール
- Codex / 外部サービスのログイン
- `known_diffs.yaml` への追加(ADRを書いた上で、次の人間レビューで承認)
- クラスタ削除・DBのデータ削除
