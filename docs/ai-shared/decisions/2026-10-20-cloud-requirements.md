## 2026-10-20: クラウド化の目的と、構成に入れる要件をユーザーが決めた(issue #149。cloud-design.md §8 の続き)
Decision: クラウド化の目的は「**Mac が寝ていてもアプリが動くこと**」。構成案は予算 3 段階(最安・月 3,000 円・理想形ラボ)× AWS / GCP / マルチクラウドで比べ、
次の要件を全案に入れる。
- **TiDB は使う・使わないを切り替えられる**ようにする(record・team の DB を TiDB と MySQL 互換のマネージド DB で差し替える。MySQL で動くかは migrate とテストで要検証。cloud-design.md §2.3)
- **DB はクラウドのマネージドサービス**を使う方向(RDS / Aurora / Cloud SQL など。予算段階ごとに候補を比べる)
- **CD は Terraform(インフラ)+ Argo CD(アプリ)**の 2 段にする。Terraform がクラスタと Argo CD の土台までを作り、アプリの配備は Argo CD に統一する(今の `make deploy-latest` の手動配備をクラウドでは使わない)
- **issue #237(Argo CD 管理の balance・speed が read model を持たず 503)を解決する**
- **監視を入れる**
- **calc の起動順依存(起動時に pokedex からマスタを取る)を解消する**
- **単一障害点(gateway 1 台など)を解消する**
- 文書を最新にする(この日に CLAUDE.md・architecture.md を更新)
- 引き続き: 非公開(Tailscale。ADR-0210)、apply・destroy は人間だけ、上限 3,000 円で開始(cloud-design.md §8 の 2026-10-11 回答)
Reason: ユーザーとの構成検討(クラウドセッション)。
Impact: cloud-design.md §8 に回答を追記。マネージド DB と単一障害点の解消は月 3,000 円の段階では費用が収まりにくいので、段階ごとに「どこまで満たすか」を構成案で示して決める(未決)。Terraform レーンの新設は構成が決まってから。

追記(2026-10-22): 翌日の **ADR-0814(2026-10-21 ユーザー決定)** が、この記録の DB の項目より新しい決定として優先する。
クラウドでも requirements.md §5 の技術構成を保ち、record・team の DB は**クラスタ内の TiDB が既定**(TiDB Cloud は条件付きで許容)、
MySQL へ寄せる案は採らない。pokedex の MySQL はクラスタ内が既定で、マネージド MySQL は運用負担の軽減としてのみ許容。
この記録の「TiDB を使う・使わないを切り替える」「DB はマネージド」は、ADR-0814 との関係をユーザーに確認する(未決)。

追記(2026-10-22。ユーザー回答): ADR-0814 との関係は **B** に決定。record・team の DB は**クラスタ内の TiDB を既定**とし、
**TiDB Cloud(同じ技術のマネージド版)へ切り替えられる**ようにする(接続先を overlay の Secret・環境変数で差し替える。アプリのコードは変えない)。
MySQL へ切り替える案(C)は採らない。ADR-0814 の範囲内なので ADR の変更は不要。TiDB Cloud の接続口が公開であること・データの置き場所は、
使う前に確認する(ADR-0814 §3 の条件)。
