## TB: タイプバランスチェッカー(タイプバランスレーン。設計は docs/type-balance-design.md)
- [x] TB0 基盤(型・相性コア・HTTP・Docker/Kustomize・Argo CD・単体テスト)
- [x] TB1 防御タイプバランス
- [x] TB1b 相性表を P1-13 のデータ
- [x] TB2 攻撃範囲(ADR-0016)
- [x] TB3 特性
- [x] TB4 仮想敵診断(ADR-0400)
- [x] TB5 おすすめタイプと該当ポケモン
- [x] TB 整備(2026-09-22)
- [x] TB 実データの配線
- [x] TB6 技範囲チェッカー
- [x] issue #108 のタイプバランス分(ADR-0138): balance・speed が read model と同じディレクトリの `metadata.json` から dataVersion を読み、起動ログと `/healthz` の任意の `dataVersion` に出す(欠落は版不明で起動継続、不正は起動エラー)。openapi は additive、web・iOS の生成物を更新。`check-master-version.sh` の healthz 参照への切替はデータレーン側
- [x] Codexレビュー issue #105 対応
- [x] 全体レビュー issue #263・#292 対応
- [x] issue #298

### ブロッカー(タイプバランスレーン)
(なし。Argo CD の実同期は 2026-09-22 に解消)

