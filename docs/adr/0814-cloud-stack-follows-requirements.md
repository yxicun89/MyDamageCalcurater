# ADR-0814: クラウドは要件(requirements.md §5)の技術構成で組む

- 状態: 採用(2026-10-21。ユーザー決定)
- 日付: 2026-10-21
- 関連: requirements.md §1・§5、ADR-0211(TiDB)、ADR-0212(NATS)、docs/plan/cloud-design.md、ADR-0812・0813(予定。クラウドの選定と費用ガード)

## 背景

cloud-design.md は、費用を抑えるため、データベースを MySQL に寄せる・マネージド MySQL にする(B・C・D 案)など、
要件の技術を別のものに置き換える案を含んでいた。しかしこのプロジェクトの目的の 2 つ目は
「フロント〜バックエンド〜DB〜インフラ(Kubernetes)まで通して**学ぶ**」ことで、requirements.md §5 の技術
(TiDB・NATS JetStream・Kubernetes・Kustomize + Argo CD・kube-prometheus-stack と Loki・Tailscale)は、
性能の必要ではなく**使ってみたい技術**として選んでいる。ユーザーは、クラウドでもその構成を保ちたい。
一方、フロントとバックの言語(Go・TypeScript/React・Swift)を変えると手戻りが大きすぎる。

## 決定

1. **クラウドの技術構成は requirements.md §5 に合わせる**。費用や手間を理由に、DB・メッセージング・k8s・デプロイ方式・監視を別の技術へ置き換えることを、既定にしない。
   置き換える場合は、ユーザーの承認と、requirements.md §5 を変える ADR を先に作る。
2. **言語は変えない**: バックエンドは Go + Echo、Web は TypeScript + React(Vite)、iOS は Swift(SwiftUI)、オフライン計算は Go → WASM のまま。
3. 技術ごとの、クラウドでの扱い:

| 要件の技術 | クラウドでの形 | cloud-design.md の候補との関係 |
|---|---|---|
| Go + Echo のマイクロサービス | 今のイメージをそのまま動かす | 変更なし |
| マスタ DB: MySQL | クラスタ内の MySQL(StatefulSet)。マネージド MySQL は、同じ MySQL 互換の運用負担の軽減としてのみ許容 | C は「MySQL のまま」なので許容。D(Aurora)は要検討 |
| ユーザーデータ DB: TiDB | **クラスタ内の TiDB(PD・TiKV・TiDB)**。TiDB Cloud は同じ技術のマネージド版として、データの置き場所と公開の確認後に許容 | **A を既定**。B(MySQL へ寄せる)・G(SQLite)は採らない。F(自宅に残す)は段階 1 の一時的な形としてのみ |
| NATS JetStream | クラスタ内の StatefulSet | 変更なし(推奨のまま) |
| Kubernetes(k3d) | 小さな VM 上の k3s(今の k3d とほぼ同じ)。学習のため EKS/GKE はラボで触る | 変更なし |
| Kustomize + Argo CD | `overlays/cloud` を作り、Argo CD が同期する | 変更なし |
| 監視: kube-prometheus-stack・Loki | クラウドでは入れる(メモリは VM の大きさで確保する)。ローカルは 2026-10-09 の決定のとおり入れない | 段階 2 の監視欄を「軽いもの」から「要件の構成」に改める |
| 外部接続: Tailscale | VM を tailnet に参加させ、公開の入口を作らない | 変更なし |

4. この方針の費用の影響: TiDB と監視をクラスタ内に置くため、**VM は 8GB 級以上**になる(段階 2 の 2a・2b)。
   月額は税込 約 7,600〜11,800 円の見込みで、現在の月額上限 3,000 円を超える。**上限の見直しか、使う時だけ起動する運用(夜間・不使用時の停止)のどちらかを、ユーザーが決める**(cloud-design.md §8 の質問に追加)。

## 影響

- cloud-design.md の §0 の DB の行、§2.3、段階 2 を、本 ADR に合わせて直す(本 PR)。
- 段階 1(record・team を自宅に残す)は、段階 2 へ進む前の一時的な形として残す。
- コードの変更は不要(record・team は元から TiDB 前提)。
- 「MySQL で動くかの検証」(cloud-design.md §2.3)は、必須ではなくなる。
