# ADR-0001: 技術スタック

- 状態: 承認
- 日付: 2026-09-21

## 決定
| 領域 | 選定 | 理由 |
|---|---|---|
| バックエンド | Go + Echo のマイクロサービス | 好み・学習目的。k8s で動かす |
| API | REST + OpenAPI 仕様先行 | Go/TS/Swift のコードを1つの仕様から生成 |
| 計算エンジン | 純粋な Go パッケージ | calc-svc と WASM の両方で同じコードを使う |
| マスタDB | MySQL + sqlc | 読み中心、業務で慣れている |
| ユーザーデータDB | TiDB | NewSQL を試す。MySQL 互換で sqlc がそのまま使える |
| 非同期 | NATS JetStream | 計算と保存の障害を分離 |
| Web | Vite + React + TS | 静的配信、WASM でオフライン計算 |
| iOS | SwiftUI | iOS のみ。Liquid Glass・シェーダー演出 |
| k8s | k3d(Mac) | クラウド移行時も k3s 系で構成を保てる |
| 画像 | MinIO + gateway /assets | 同梱しない。無ければエンブレム |

## 却下した案
- Flutter: Android 不要のため利点が薄い
- 計算をサーバーのみ: Mac が止まると使えない → WASM 併用で解決
- ORM(GORM/ent): SQL を直接書いて学ぶため sqlc を選択
