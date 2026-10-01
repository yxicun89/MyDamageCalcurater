# ADR-0410: タイプバランス設計書の書き換えと旧版の履歴(issue #260)

- 状態: 採用(2026-10-01。issue #260 のタイプバランスレーン分。ドキュメントのみ)
- 日付: 2026-10-01
- 関連: docs/type-balance-design.md(現在の設計)、ADR-0012・0014・0015・0018・0210・0405・0408・0409、docs/ai-shared/COORDINATION.md、DECISIONS.md

## 背景
`docs/type-balance-design.md` は 2026-09-21 の「Claude にレビューを依頼する」ための文書のまま残り、実装(TB0〜TB6・メトリクス・GitOps・同時実行上限)
の後追いになっていなかった。存在しない `/api/damage`、別名 `pokecalc-kit-v2`、Claude/Codex の役割分担、レビュー依頼、Codex の実装開始条件、
実装済みの事項を含む「未決事項」などが残っていた。レーン制(COORDINATION.md 2026-09-21)以後は、これらは当てはまらない。

## 決定
1. 設計書を、docs/speed-design.md に揃えた「現在の設計」(目的・位置づけ・構成・計算・API・段階・データ・運用・GitOps・UI・未対応)に書き換える。
   根拠は `services/balance` の実コード・`services/balance/api/openapi.yaml`・ADR・plan.md。未対応は未対応と明記する。
2. 旧版の役割分担・レビュー依頼・AI 間の共有ルール・Codex の実装開始条件は、現在の運用(AGENTS.md・COORDINATION.md のレーン制)に置き換わったため、設計書から外す。
   履歴は Git の履歴(2026-09-21 版)と、この ADR の下の記録に残す。
3. ADR を含む既存文書が旧版の節番号(§4・§6・§7・§8・§10・§14・§16 など)を参照している。ADR は書き換えない。新版でも
   §6 は段階(TB1〜TB6)、§10 は倍率の表示のままとして、画面側の参照が意味を保つようにした。それ以外の旧参照は下の対応表で読み替える。

## 旧版の節と、現在の扱い
| 旧版 | 内容 | 現在 |
|---|---|---|
| §1・§11・§13・§15 | Claude / Codex の役割分担・AI 間の共有ルール・Codex の実装開始条件 | 廃止(レーン制。AGENTS.md・COORDINATION.md) |
| §2・§3 | 目的・アーキテクチャ・k8s を採用する理由 | 新 §1・§2・§3 |
| §4 | 技術スタック・GitOps / Argo CD・永続化 | 新 §3・§9(永続化は新 §11) |
| §5 | 計算コアの設計 | 新 §3・§4 |
| §6 | 段階実装(TB0〜TB6) | 新 §6(表)・§4 |
| §7 | 倍率表現(整数・基準 4) | 新 §4 |
| §8 | API 案 | 新 §5(契約は openapi.yaml が正。パスは `/api/balance/v1/...`) |
| §9 | マスタ・外部 API・レートリミット | 新 §7 |
| §10 | UI 案 | 新 §10 |
| §12 | Claude へのレビュー依頼 13 項目 | 廃止(2026-09-21 に実施。結果は `docs/ai-shared/claude-review.md` と ADR-0012・0014) |
| §14 | Git / GitOps 運用 | 新 §9。ブランチ運用は AGENTS.md |
| §16 | 未決事項 | 下の決着表 |

旧版の次の記述は誤り・古いものだった: Ingress の `/api/damage`(実際のダメージ計算は別サービス `/api/calc`)、別名 `pokecalc-kit-v2`、
「Codex は次回レートリミット回復後に開始」。

## 旧 §16「未決事項」の決着(実装済みの事実)
| 旧未決事項 | 決着 |
|---|---|
| 同一リポジトリか別リポジトリか | 同一リポジトリの `services/balance/`(ADR-0012) |
| Pokemon マスタの正本 | pokedex。pokedex export の read model を balance が起動時に読む(ADR-0012・0014・0403) |
| request は `pokemonId` のみか | `pokemonId` のみ(ADR-0014 §1。DECISIONS 2026-09-21) |
| 共通 Go モジュールを作る時期 | 作っていない。サービスごとに go.mod(ADR-0012。`httpmetrics` は複製。ADR-0406 §2) |
| 永続化の DB | balance は DB を持たない(保存は team-svc の責務) |
| plain YAML / Kustomize / Helm | Kustomize(DECISIONS 2026-09-21) |
| Argo CD Application をサービス単位で分けるか | サービス単位(`pokecalc-balance`・`pokecalc-speed`)。AppProject `pokecalc` で限定(ADR-0408) |
| 自動 sync / prune / selfHeal | 無効(手動 sync)。`scripts/gitops/check-gitops.sh` が有効化を検出して失敗する(ADR-0408 §4)。有効化は未決のまま |
| 本番クラウド(EKS / GKE) | 未決。クラウド公開はしない方針(ADR-0210) |
| 本番デプロイの時期 | ローカル(k3d)での検証まで。クラウドへは未実施 |

## 結果
- 設計書を読めば、実装済みの範囲・段階の完了状況・未対応事項が分かる。
- コード・API・他の ADR は変えない。
