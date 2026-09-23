# ADR-0109: importer の手動実行と CronJob の相互排他(issue #106)

- 状態: 採用(issue #106 の仕様。spec-writer 起草、implementer が実装、critic PASS)
- 日付: 2026-09-23
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #106、ADR-0100(§9 k3d の MySQL・Secret・Job の流儀)、ADR-0101(§1 構成・§11 CLI の終了コード規約の初出)、
  ADR-0103(§1 Reconcile の裁定)、ADR-0104(importer の CronJob 全体の設計。§2 cronjob.sh・§3 終了コード・
  §5 CronJob マニフェスト・`concurrencyPolicy: Forbid` の意図)、CLAUDE.md 絶対ルール 2/6/7

## 背景

ADR-0104 §5 は `concurrencyPolicy: Forbid` を「手動実行(`make import-k8s`)と重なっても同時に投入しない」設定として
採用したが、これは誤りだった。Kubernetes の `concurrencyPolicy` は**同じ CronJob が作る Job 同士**にしか働かない
([公式ドキュメント](https://kubernetes.io/docs/concepts/workloads/controllers/cron-jobs/#concurrency-policy))。
`make import-k8s`(`kubectl create job --from=cronjob/pokedex-import`)が作る Job は CronJob の管理下に入らない独立した
Job なので、`Forbid` はこれを拒否しない。

結果として、CronJob が作った定期 Job が実行中に `make import-k8s` を叩く(または逆)と、2つの Pod が同時に
`pokedex-import-cache` PVC の `/app/data/generated` を読み書きしうる。両方とも
`tools/importer/cronjob.sh`(取得 → 上流の検出 → 照合・投入)を最後まで走らせる可能性があり、取得キャッシュの
書き込み競合・`reports/`・`upstream/latest.json` の競合、DB への2つの全置換投入の競合(deadlock・不要な再試行)を
起こしうる(`Apply` 自体は1トランザクションで原子的だが、2つの `Apply` が別々に走ることは防げない)。

## 決定

### 1. ロックは `cronjob.sh` だけに実装する(CLI へのフラグ追加はしない)

`tools/importer/cronjob.sh` は「取得(Node)→ 上流の検出(Node)→ 照合・投入(Go)」を1つのコンテナ・1つの
プロセスツリーで順に呼ぶ構造になっている(ADR-0104 §2)。したがって sh レベルで排他すれば、Node の
キャッシュ書き込みから Go の DB 投入まで工程全体を1本のロックで覆える。`services/pokedex/cmd/import` に
ロック用のフラグを追加する必要は無い(CLAUDE.md 絶対ルール2「engine は純粋に保つ」とは別の理由だが、
importer CLI も「入力を読んで DB に書く」役割に留め、プロセス間排他という別の関心事を持ち込まない)。

却下: Go の `pokedex-import` 側にロックを実装する(Node の `fetch.mjs` のキャッシュ書き込みはロックの外に
残ってしまい、issue の根本(取得キャッシュの競合)を塞げない)。

### 2. OS のアドバイザリロック(`flock`)を共有 PVC 上のファイルで取る

- イメージには busybox の `flock` アプレットが既に入っている(追加パッケージ不要。確認: `node:26.9.0-alpine`
  の busybox 1.37.0)。
- `cronjob.sh` の冒頭、**`fetch.mjs` を呼ぶ前**に、ファイルディスクリプタ経由で非ブロッキング取得する:

```sh
exec 9>"$LOCK_FILE"
flock -n 9 || {
  echo "cronjob: 別の import が実行中(ロック $LOCK_FILE を取得できない)。今回は諦める" >&2
  exit 1
}
```

- ロックファイルは共有 PVC 上(`mountPath: /app/data/generated` の配下)に置く。CronJob の Job も
  `make import-k8s` の手動 Job も**同じ PVC・同じ podTemplate**(`kubectl create job --from=cronjob/...` は
  jobTemplate をコピーする)を使うため、どちらのプロセスも同じロックファイルを見る。
  既定パス: `${IMPORT_APP_DIR:-/app}/data/generated/.import.lock`。
- 環境変数でテストから差し替えられるようにする(本番の `/app` に書き込めないネイティブ Go のテストのため):
  - `IMPORT_LOCK_FILE`: ロックファイルの絶対パス。既定は上記。
  - `IMPORT_APP_DIR`: `/app` 配下のパス(`tools/importer/fetch.mjs`・`tools/importer/check-upstream.mjs`・
    `pokedex-import` バイナリ)の基点。既定 `/app`。
  この2つの環境変数名は `services/pokedex/importer/cronjob_lock_test.go` が直接使うテスト契約であり、
  実装者が別の名前に変えるとテストが通らない。

### 3. ロックは fd で保持し、`exec` を挟んでも Go プロセスまで引き継がれる。stale lock 対策は不要

- `exec 9>"$LOCK_FILE"` で開いた fd 9 は、シェルが `CLOEXEC` を立てない限り、スクリプト末尾の
  `exec /app/pokedex-import ...`(プロセス置換)後も同じプロセスに残り続ける。したがって「取得 → 上流の検出 →
  投入」の全工程が同じロック保持プロセスの中で進む。
- ロックはカーネルが管理する advisory lock なので、プロセスが正常終了・異常終了・`SIGKILL`・ノード停止に伴う
  Pod の強制終了のどれで終わっても、プロセスが持つ全ての fd と一緒に**自動的に解放**される。ロックファイル自体は
  PVC 上に残り続けるが、中身や存在そのものは次の `flock` の可否に影響しない(ロックの成否はファイルの中身では
  なくカーネルの lock table で決まる)。**stale lock は原理的に発生しない。**
- 却下: `mkdir` によるロック(ディレクトリの作成をロック獲得とみなす方式)。`mkdir` は成功すると
  ディレクトリが残るだけで、プロセスが `SIGKILL` された場合に「誰も持っていないのに存在する」ディレクトリ
  (stale lock)が残り、次回の起動時に人間が消さないと永久に排他されたままになる。issue の受け入れ条件
  「Pod の強制終止・deadline 超過・ノード停止後にロックが自動解放され、恒久的な stale lock を残さない」を
  満たせない。

### 4. ロック取得失敗の終了コードは 1(既存の終了コード規約に乗せる。CronJob マニフェストの変更は不要)

- `services/pokedex/cmd/import/main.go` の既存規約(ADR-0104 §3): 0 成功 / 1 再試行で直りうる失敗 / 2 使い方誤り /
  3 人間対応が必要。ロック取得の失敗は「今は別のプロセスが処理中なだけで、後で再試行すれば直りうる」失敗であり、
  1 に分類するのが規約と整合する。
- `deploy/k8s/base/pokedex/cronjob-import.yaml` の `podFailurePolicy` は既に終了コード 2・3 だけを `FailJob`
  (再試行しない)にしており、1 は `backoffLimit`(2)で自動再試行される。**この設計により、cronjob.sh がロックの
  失敗を exit 1 で終わらせるだけで、CronJob マニフェストの変更は不要になる。**
  - 負けた側が CronJob の定期 Job だった場合: 最大2回まで自動再試行され、その間に相手が終わっていれば
    次の試行で成功する。`activeDeadlineSeconds`(3600秒)の範囲であれば十分な余地がある。
  - 負けた側が `make import-k8s` の手動 Job だった場合: 同様に自動再試行される。それでも失敗が続くなら
    (2回とも競合し続けるなら)人が `kubectl get jobs` で見て、時間をずらして再度 `make import-k8s` を叩く。
- 却下: ロック取得の失敗を終了コード 3(人間対応が要る)にする。一時的な競合を「人間対応が要る恒久的な失敗」
  として扱うのは過剰で、CronJob の自動再試行という既にある仕組みを使わない理由が無い。

### 5. `concurrencyPolicy: Forbid` との役割分担

- `Forbid` は引き続き維持する。ただし担う役割を「同じ CronJob が作る Job 同士の重複実行の防止」(例:
  スケジュールされた実行が長引いている間に次のスケジュールが来た場合)に限定する。
- `flock` によるアプリ側の排他は、CronJob の Job・`make import-k8s` の手動 Job・将来増えうるどんな起動経路
  (例えば別の CronJob や別の手動コマンド)であっても、同じ PVC の `fetch → upstream check → reconcile → apply`
  を横断して排他する。**この2つは互いの代わりにならない**(`Forbid` だけでは手動 Job を防げず、`flock` だけでは
  「CronJob が Job を作ること自体」を止められない。両方があって初めて issue の受け入れ条件を満たす)。

### 6. 却下したその他の案

- **DB 上のロック行**(`data_versions` へのロック用カラム追加、MySQL の `GET_LOCK()` など): 排他したい対象は
  DB だけでなく `fetch.mjs` のキャッシュ書き込み(ファイル I/O)も含むため、DB のロックだけでは issue の根本原因
  (取得キャッシュの競合)を塞げない。加えて、migrate が済んでいない DB(`ErrSchemaNotReady`)ではロック用の
  テーブルにすら触れず、ロック機構自体が DB の前提を必要としてしまう(現状 `cronjob.sh` は DB 接続なしで
  `fetch.mjs`・`check-upstream.mjs` を走らせられる。この独立性を失いたくない)。
- **Kubernetes Lease API**(`coordination.k8s.io`): `cronjob-import.yaml` は
  `automountServiceAccountToken: false`(ADR-0104 §5)を明示しており、k8s API を呼ばない設計を意図的に選んでいる。
  Lease を使うには ServiceAccount・RBAC・k8s client の追加が要り、コストに対して得るものが `flock` と変わらない。
- **CronJob 側の追加設定で防ぐ**(`activeDeadlineSeconds` の調整など): issue の根拠のとおり `Forbid` は手動 Job
  を対象にできず、他の CronJob 側設定にも同じ限界がある。アプリ側の排他が必須。

## 受け入れ条件

1. ロック取得に失敗した2つ目のプロセスは、`fetch.mjs`(取得キャッシュへの書き込み)・`pokedex-import`
   (DB 投入)のどちらも一切実行せず、終了コード 1 で終わる。
2. ロック取得に成功した1つ目のプロセスは、ロックが無い場合と同じように最後まで(取得 → 上流の検出 → 投入)進む。
3. ロックファイルの中身・存在は、プロセスの終了理由(正常・異常・`SIGKILL`)によらず、終了後は次のプロセスの
   ロック取得を妨げない(stale lock にならない)。
4. ロック競合をログに報告するとき、DSN 等の秘密を含まない。
5. `deploy/k8s/base/pokedex/cronjob-import.yaml` の変更は不要(終了コード 1 は既存の `podFailurePolicy`/
   `backoffLimit` の対象のまま)。
6. 既存の `services/pokedex/importer/cronjob_layout_test.go` の主張(`TestCronJobScriptOrder` を含む)を弱めない。

## テスト

| AC | テスト |
|---|---|
| 1, 3, 4 | `services/pokedex/importer/cronjob_lock_test.go`: `TestCronJobLockRejectsConcurrentRun`(統合。2プロセス同時起動) |
| 2, 3 | 同: `TestCronJobLockReleasedAfterExit`(1つ目の終了後、2つ目が取得できる。stale lock にならないことの確認) |
| 1, 2 | `services/pokedex/importer/cronjob_layout_test.go`: `TestCronJobScriptLocksBeforeFetch`(静的。`flock -n` の使用・`fetch.mjs` より前であること・`IMPORT_LOCK_FILE`/`IMPORT_APP_DIR` の環境変数対応・`mkdir` 方式を使っていないこと) |
| 5 | 同ファイルの `TestImportCronJobJobSpec`(既存。`podFailurePolicy` が終了コード1を `FailJob` にしないことは既に固定済みで、本 ADR による変更が不要であることの裏付けにもなる) |
| 6 | 既存テストへの変更なし(critic レビューで確認) |

- `cronjob_lock_test.go` は `sh`・`flock` が無い環境では `t.Skip` する(`services/gateway/deploytest/smoke_test.go` の
  `curl` の流儀と同じ)。macOS の既定の `/bin/sh` に `flock` は入っていない(busybox/util-linux 由来のため)。
  ローカルで実行するには `brew install util-linux`(または `flock` を含む別の提供)などで補う必要がある。
  CI・importer イメージ(Linux, busybox)では素の `make test` で走る。

## 影響

- 変更されるファイル: `tools/importer/cronjob.sh` のみ(実装は implementer)。
- 変更されないファイル: `services/pokedex/cmd/import/main.go`(CLI フラグ・終了コード規約とも既存のまま)、
  `deploy/k8s/base/pokedex/cronjob-import.yaml`、`Makefile`(`import-k8s` はそのまま)。
- ADR-0104 §5 の「手動実行(`make import-k8s`)と重なっても同時に投入しない」というコメントは
  `concurrencyPolicy: Forbid` の限界(同じ CronJob の Job 同士にしか効かない)を正確に書けていなかった。
  ADR-0104 に本 ADR を参照する追記を行った(下記「追記」参照)。

## 限界

- ロックは `flock` の POSIX advisory lock に依存する。ローカル k3d の `local-path` PVC(ノードのローカル
  ディスク上の hostPath)では問題にならないが、将来 cloud overlay で NFS 系の `ReadWriteMany` PVC に切り替える
  場合、NFS の `flock` サポート(サーバー・クライアントの設定に依存)を別途確認する必要がある(v1 の cloud
  overlay は CronJob 自体を `suspend: true` にしており対象外。ADR-0104 §8)。
- ロックの粒度は「importer 全体(1プロセス)」であり、取得・上流検出・投入を個別に並列化する余地は無くす
  (ADR-0104 §2 が既にこれらを1つのプロセスにまとめており、本 ADR はその前提を維持するだけで新たな制約は増やさない)。
- 2つ目のプロセスが `backoffLimit` の再試行でも競合し続ける最悪ケース(理論上は起こりうるが、週1回のスケジュールと
  手動実行の頻度を考えると現実的ではない)は、`BackoffLimitExceeded` で Job が失敗する。ここへの追加対策
  (指数バックオフでの待機など)は v1 では行わない。

## 実装時の申し送り(implementer 向け)

- k3d での動作確認手順の下書きを `docs/runbooks/data.md` に追加した(「7. 手動実行と CronJob の重複を確かめる」)。
  実際の実行(kubectl でのクラスタ操作)は implementer またはユーザーが行う。
- ADR-0104 §5 のマニフェスト内コメント(`deploy/k8s/base/pokedex/cronjob-import.yaml` の
  `concurrencyPolicy: Forbid` の直前のコメント)も、実装のついでに本 ADR を参照する形へ更新するのが望ましい
  (本 ADR では yaml ファイルの変更は行っていない)。
