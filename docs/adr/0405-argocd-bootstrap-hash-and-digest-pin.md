# ADR-0405: Argo CD 導入物(install manifest・同梱イメージ)をハッシュと digest で固定する

- 状態: 採用(2026-09-23。Codexレビュー issue #105 への対応。ユーザーが「実装を進めてほしい」と承認)
- 日付: 2026-09-23
- 関連: ADR-0018(balance の Argo CD 導入。TB0)、ADR-0605(speed の Argo CD 共有。SP5)、docs/runbooks/balance.md、docs/runbooks/speed.md、
  issue #105

## 背景
ADR-0018 で入れた Argo CD は、公式リリースタグ配下の `install.yaml` を `kubectl apply -n argocd --server-side -f <raw GitHub URL>` で
直接クラスタへ適用している(`docs/runbooks/balance.md` §3、`docs/runbooks/speed.md` §5 に、ほぼ同一の手順が重複して存在)。
URL はタグ名を含むがタグ自体は内容ハッシュではなく、取得内容を検証する手段が手順に無い。manifest が参照する3イメージ
(argocd-server・dex・redis)もタグ指定のみで digest 固定されていない。Argo CD はクラスタ管理者権限を持つため、取得経路・上流タグ・
イメージのいずれかが差し替わった場合、無検証でクラスタに適用・実行されてしまう。

balance の GitOps overlay(`deploy/k8s/overlays/gitops`)は balance 自身のアプリイメージをすでに digest で固定しており(ADR-0018 §3)、
`check-gitops.sh` がその digest 形式を検査する前例がある。この前例を Argo CD 自身の導入物にも広げる。

## 決定

### 1. 導入手順を共有スクリプト1本にする
新設 `scripts/argocd-bootstrap.sh`(リポジトリルート。`doctor.sh`・`db-local-up.sh` と同じ「横断的なインフラ準備」の置き場)を、
balance・speed の両 runbook から呼ぶ1本の手順にする。balance/speed それぞれの `scripts/` 配下には置かない(ADR-0605 が明言する
「balance のスクリプトを写経する」慣習をこの手順には適用しない。値を二重管理しないという issue の受け入れ条件のため)。

- `cd "$(git rev-parse --show-toplevel)"` から始める。
- `mktemp -d` で一時ディレクトリを作り、`trap 'rm -rf "$tmp_dir"' EXIT` で終了時に必ず削除する。
- 取得元 URL は **タグではなくコミットSHA固定**にする: `https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_INSTALL_COMMIT}/manifests/install.yaml`
  (`ARGOCD_INSTALL_COMMIT=c9c369efcc5b2a0bd720803f8d14a1c3eaddf579`。v3.5.3 タグが指すコミット。2026-09-23 に
  `https://api.github.com/repos/argoproj/argo-cd/git/refs/tags/v3.5.3` で確認)。タグ名は比較用のコメントとしてのみ残す。
- 取得した manifest の SHA-256 を `EXPECTED_INSTALL_YAML_SHA256`(`7efe2d6bbc03f63623640f1e4198f16c84009d510fb810ef71e56df1b7614ba9`。
  2026-09-23 に取得して確認した値)と比較し、不一致なら適用前に非0で終了する。
- 取得した manifest 内の3イメージをタグ参照から digest 参照へ書き換えてから適用する(`sed` による文字列置換。値は下記定数)。
  書き換え後、3イメージすべてが `image: <repo>@sha256:[0-9a-f]{64}` の形になっていることを検査してから適用する
  (`check-gitops.sh` の digest 正規表現検査と同じ考え方)。
  - `ARGOCD_IMAGE_DIGEST=sha256:dd3f47d5a5e4da563a7a398506e892481b358a7cec50abdf320c71aa55904bfa`(`quay.io/argoproj/argocd:v3.5.3`)
  - `DEX_IMAGE_DIGEST=sha256:8499afd690c437f52301efd2b05b2455da5bd2dfc20332cd697dc9937f808462`(`ghcr.io/dexidp/dex:v2.45.1`)
  - `REDIS_IMAGE_DIGEST=sha256:08ad0b1d280850169a790dba1393ff7a90aef951fc19632cf4d3ce4f78e679ba`(`public.ecr.aws/docker/library/redis:8.2.3-alpine`)
- namespace 作成は ADR-0605 の speed 手順にならい冪等にする(`kubectl get namespace argocd 2>/dev/null || kubectl create namespace argocd`)。
  balance が先に導入済みでも speed が壊さず再実行できることを維持する。
- 適用は `kubectl apply -n argocd --server-side -f -`(書き換え後の manifest を標準入力から)。適用後 `kubectl -n argocd rollout status
  deployment/argocd-server --timeout=300s` で待つ(既存手順と同じ)。
- 上記の定数(コミットSHA・期待ハッシュ・3 digest)はこのスクリプト1箇所にだけ持つ。値の更新(Argo CD の版を上げるとき)はこのファイルの
  差分としてコードレビューの対象になる。

### 2. 自動検査
`scripts/argocd-bootstrap_test.sh`(または同等の自動テスト。実装は spec-writer/implementer が既存のテスト方式に合わせて決める)で、
少なくとも次を検出できることを確認する。
- 取得した manifest の内容が期待ハッシュと不一致なら、スクリプトが `kubectl apply` を呼ぶ前に非0で終了する
  (ネットワーク越しの実取得はテストで行わず、取得元 URL/コマンドをテストから差し替えられるようにする。例: 取得コマンドを関数に切り出し、
  テストではローカルの改ざん済み固定ファイルを返す関数に差し替える)。
- 書き換え後の3イメージのいずれかが digest 形式(`^[^@]+@sha256:[0-9a-f]{64}$`)になっていなければ、適用前に非0で終了する。
- `EXPECTED_INSTALL_YAML_SHA256` や3 digest 定数が空文字なら、適用前に非0で終了する。
- 実クラスタへの `kubectl apply` 自体はテストで実行しない(ADR-0018 §2 の「クラスタ削除・変更は人間の確認のもとで」の精神を踏襲し、
  ここでは検証ロジックだけを自動テストの対象にする。実際の適用確認は k3d 上での手動実行に委ねる)。

### 3. runbook の書き換え
`docs/runbooks/balance.md` §3・`docs/runbooks/speed.md` §5 の、生の `kubectl apply -f <raw GitHub URL>` を含む重複ブロックを、
どちらも `./scripts/argocd-bootstrap.sh` の1行呼び出しに置き換える。speed 側の「すでに入れていればこの節はとばす」という注記
(ADR-0605 の「speed は自分の Argo CD を入れない」)は残す(スクリプト自体が冪等なので、記述だけ簡潔になる)。

### 4. 変えないこと
- Argo CD の manual sync・selfHeal 無効・レーンごとに専用 Application・read-only PAT をユーザー自身が登録、という既存方針(ADR-0018 §6)は変えない。
- balance/speed それぞれのアプリイメージの digest 固定(`deploy/k8s/overlays/gitops`)は対象外(すでに固定済み)。
- `application.yaml` の repoURL 埋め込み方式・`check-gitops.sh` の既存検査は変えない。

## 影響と制約
- Argo CD の版を上げるときは、このスクリプトのコミットSHA・期待ハッシュ・3 digest を明示的に更新する必要がある(自動追従しない。
  これは意図的: 無検証追従を防ぐという issue の目的そのもの)。
- 3イメージの digest は Argo CD 側のリリースでベースイメージが変わると古くなる。次回の版更新時に `crane digest <image:tag>` で取り直す。
- スクリプトは `crane`(または同等の digest 取得手段)を実行時には必要としない(定数として埋め込み済みのため)。`curl`・`shasum`・`sed`・`kubectl` のみに依存する。

## 却下した案
- balance/speed それぞれの `scripts/` に手順を複製する(ADR-0605 の既存慣習): issue の受け入れ条件「同じ共有導入スクリプトを呼び、
  固定値を二重管理しない」に反する。Argo CD 自体の導入は balance/speed どちらの所有物でもない横断的インフラなので、ルート `scripts/` に置く。
- `crane`/`skopeo` を実行時に呼んで最新 digest を都度解決する: 「上流のタグや配布内容が後から変わっても無検証で適用しない」という
  issue の目的に反する(都度解決は結局タグ追従と同じ無検証適用になる)。digest は版更新のたびに人間/レビューが明示的に更新する運用にする。
- 期待ハッシュを別の JSON/YAML 設定ファイルに切り出す: 定数は1ファイル(このスクリプト)に留め、変更差分を追いやすくする(check-gitops.sh の
  digest 検査と同じ「シェルスクリプト中の定数を直接検査する」流儀に揃える)。
