# ADR-0104: importer の CronJob(週1回)と make import の運用(P2-2d)

- 状態: 提案(P2-2d の仕様。spec-writer 起草、implementer が実装、critic がレビュー)
- 日付: 2026-09-22
- 関連: plan.md P2-2d、ADR-0101(§1 構成・63行 CronJob のイメージは Node と Go の両方・§9 版と冪等な投入・§10 P2-2d の範囲・§11 CLI)、
  ADR-0100(111行 checksum が一致すれば取り込まない・§9 k3d の MySQL・Secret・Job の流儀)、ADR-0102(イメージはタグ+digest で固定)、
  ADR-0103(§1 CronJob も Reconcile の入口を使う・§12 固定した版)、ADR-0018(ローカルの GitOps。レジストリと digest)、
  DECISIONS.md 2026-09-21(マスタ更新は CronJob で週1回・取得元の版に変化が無ければ取り込まない・手動の make import も残す)、
  COORDINATION.md(main へは PR。Argo CD が main を見る)、CLAUDE.md 絶対ルール 1/2/4/6
- 番号: データレーンの帯(0100〜)の5本目

## 背景

P2-2b/c で「取得(Node。ネットワーク)→ 照合 `Reconcile`(純粋)→ 投入 `Run`(版と checksum が同じなら何もしない)」は手動の
`make import-fetch` / `make import` で動くようになった。P2-2d では、これを k8s の CronJob で週1回動かし、ユーザー決定
「取得元の版に変化が無ければ取り込まない」を満たす。ADR-0101 §10 は P2-2d に「上流の最新版の検出・config.json の版の更新の運用」も
割り当てているが、`config.json` の版は Git で固定しており(ADR-0103 §10・§12)、CronJob が上流の HEAD を勝手に取り込むと
GitOps の「Git が正」と矛盾する。この関係を先に決める。

## 決定

### 1. 固定した版と上流の最新版 — CronJob は固定版だけを取り込み、上流の新しい版は検出して報告だけする

- **CronJob が取得・投入するのは、Git にコミットされた `data/importer/config.json` の `sources`(固定した版)だけ**。
  上流(npm の `@smogon/calc`・`smogon/pokemon-showdown` の `master`・`PokeAPI/pokeapi` の `master`)に新しい版があるかは
  検出して**ログと報告に出すだけ**で、`config.json` も DB も変えない。
- 版を上げるのは人: `config.json`(calc なら `tools/importer/package.json` の `@smogon/calc` も)をブランチで更新 →
  `make import-fetch && make import-dry-run` で照合(ADR-0103 §5 の裁定の件数・ハッシュ)が通ることを確かめる → PR → main。
  照合が通らない版は ADR-0103 §1 のとおり自動では入らない(版を上げた PR の段階で止まる)。
- ユーザー決定との整合: 「週1回」= CronJob のスケジュール。「版に変化が無ければ取り込まない」= `NeedsImport`(ADR-0101 §9)が
  **DB の `data_versions` と、固定版から作ったスナップショット・設定ファイルの版と checksum** を比べ、すべて同じなら `Apply` しない。
  したがって週1回の CronJob の実効は **(a) DB が Git の固定版と食い違っていれば直す(初回・PVC の消失後・config の変更の反映)、
  (b) 上流に新しい版が出たことを知らせる** の2つになる。「版の変化」は「Git に固定した版の変化」と読む(上流の変化は人の PR を経て
  Git の変化になる)。
- 却下: CronJob が上流の HEAD を取り込む(Git と DB が食い違い、裁定を経ない版が入る。ADR-0103 §1 の趣旨にも反する)/
  CronJob が `config.json` を書き換えて PR を自動作成する(クラスタに Git の書き込み権限を渡すことになる。v1 では過剰)。

### 2. CronJob の中身 — 1つの Job・1つのコンテナで「取得 → 上流の検出 → 照合 → 投入」を順に行う

```
tools/importer/cronjob.sh(POSIX sh。set -eu)
  1. node /app/tools/importer/fetch.mjs            # 固定版の取得。キャッシュ(PVC)にあれば取得元へ出ない。失敗 → 終了(非0)
  2. node /app/tools/importer/check-upstream.mjs || 警告だけ出して続ける   # 上流の検出。失敗しても取り込みは止めない
  3. exec /app/pokedex-import -data /app/data -upstream /app/data/generated/upstream/latest.json
                                                   # Reconcile → 報告 → RunStore(版が同じなら何もしない)
```

- **1つにまとめる理由**: 段の間の受け渡しは `data/generated/`(ファイル)で、分けても共有ボリュームが要るだけで得るものが無い。
  ログが1か所にまとまり、どこで止まったかが一目で分かる。ADR-0101 63行(イメージは Node と Go の両方)のとおり1つのイメージにする。
  却下: initContainer(Node)+ container(Go)の2段(イメージが2つになり、Job の失敗の原因がコンテナをまたぐ)/ CronJob を段ごとに分ける
  (順序の保証が無い)。
- `-force` は付けない(付けると毎週全置き換えになり「変化が無ければ取り込まない」に反する)。`-dry-run` も付けない。
- migrate は CronJob から流さない(`pokedex-migrate` Job の担当。ADR-0100 §5)。down は当然どこからも流さない。

### 3. CLI の追加(`services/pokedex/cmd/import`)

- フラグ: 既存の `-data` / `-dry-run` / `-force` に加え、
  - `-upstream <path>`: 上流の検出結果のファイル(§4)。空(既定)なら検出の表示をしない。ファイルが無い・壊れている・古いときは
    **警告を出すだけで終了コードに影響しない**(上流の検出は取り込みを止めない)。
  - `-upstream-max-age <duration>`: 既定 `24h`。`checkedAt` がこれより古い結果は `unknown`(前回の結果の使い回しを「最新」と誤らない)。
- 表示: `LoadInput` の後に上流の比較(`FormatUpstream`)を、`Reconcile` の後に `FormatSummary` を stdout に出す(`kubectl logs` で読む。
  どちらも ID・件数・版だけで名前を含まない。ADR-0103 §2)。
- 終了コード(CronJob の `podFailurePolicy` が使う。§5):

| コード | 意味 | 例 |
|---|---|---|
| 0 | 成功(投入した・版が同じなのでスキップした・dry-run) | |
| 1 | 再試行で直りうる失敗 | DB に接続できない・報告を書けない・その他の I/O |
| 2 | 使い方・設定の誤り(再試行しても同じ) | フラグの誤り・`POKEDEX_DATABASE_DSN` が無い |
| 3 | 人の対応が要る(再試行しても同じ。DB は変えない) | `ErrBlocked`(裁定の食い違い)・`ErrKeyChanged`・`ErrInvalidInput`・`ErrInvalidData`・`ErrInvalidEffect`・`ErrSchemaNotReady` |

- テストのため `run(args []string, env cliEnv) int` にし、DB への接続を `env.OpenStore` で差し替えられるようにする(§7)。
  `-dry-run` と照合の失敗・DSN が無いときは `OpenStore` を呼ばない(DB に触らない)。

### 4. 上流の検出(ネットワークは Node、比較は Go の純粋関数)

- ADR-0101 §1「Go の importer は実行時にネットワークへ出ない」を保つため、上流への問い合わせは
  `tools/importer/check-upstream.mjs`(新規)が行い、結果を `data/generated/upstream/latest.json` に書く(Git 管理外)。
  Go はそのファイルを読んで固定版と比べるだけ。ファイルが「上流の検出」の差し替え口になり、Go のテストはネットワークに触らない。
- 問い合わせ(週1回・計3回): npm registry の `@smogon/calc` の `latest`(1回)、GitHub API の `smogon/pokemon-showdown` と
  `PokeAPI/pokeapi` の `master` の commit(各1回。`Accept: application/vnd.github.sha`。未認証の上限 60 回/時に対して十分少ない。
  トークンは使わない)。リポジトリ名・ブランチ名は fetch-*.mjs と同じく Node スクリプトの先頭の定数に置く(取得元の識別子。Go に書かない)。
- ファイルの形式(厳格。未知のフィールドは拒否):

```json
{"schemaVersion": 1, "checkedAt": "2026-09-26T03:00:00Z",
 "sources": {"calc": "0.12.0", "showdown": "<40桁>", "pokeapi": "<40桁>"},
 "errors":  {"<source>": "<取れなかった理由>"}}
```

  - check-upstream.mjs は**毎回ファイルを書き直す**(一部または全部が取れなくても `errors` に理由を入れて書く。古い結果を残さない)。
  - source は `config.json` の `sources` のキーと同じ名前。同じ source が `sources` と `errors` の両方にあれば不正。
- Go の比較(`CompareUpstream`): `config.json` の `sources` の source ごとに(昇順)、`same` / `differs` / `unknown`
  (`errors` にある・検出結果に無い・`checkedAt` が古い)。source の名前は `config.json` から取り、Go に列挙しない。
  「新しい」ではなく「違う」(`differs`)とする: commit の前後はネットワーク無しでは判定できず、calc も固定版と違えば人が見る価値がある。
- `differs` はログに `UPSTREAM` の行で「source・固定版・上流の版・`data/importer/config.json` を PR で更新する」旨を出す。**Job は成功のまま**
  (失敗にすると、人が PR を出すまで毎週失敗が続き、本当の失敗と見分けにくい)。

### 5. CronJob のマニフェスト(`deploy/k8s/base/pokedex/cronjob-import.yaml`)

| 項目 | 値 | 理由 |
|---|---|---|
| `metadata.name` | `pokedex-import` | pokedex の DB への書き手(絶対ルール4。pokedex の配下に置く) |
| `schedule` / `timeZone` | `0 12 * * 6` / `Asia/Tokyo` | 週1回(ユーザー決定)。ローカルの k3d はノート PC 上で動くので、起動している可能性が高い土曜の昼にする。タイムゾーンを明示し UTC の読み替えを不要にする |
| `concurrencyPolicy` | `Forbid` | 手動実行(`make import-k8s`)と重なっても同時に投入しない(`Apply` は1トランザクションだが、取得のキャッシュの書き込みが競合する) |
| `startingDeadlineSeconds` | `518400`(6日) | クラスタが予定の時刻に止まっていても、次の予定より前に起動すれば1回だけ追いつく。7日未満にして次の予定と重ねない |
| `successfulJobsHistoryLimit` / `failedJobsHistoryLimit` | `3` / `3` | 直近の結果とログを残す |
| `suspend` | base では付けない(false) | cloud overlay だけ `true`(§8) |
| `jobTemplate.spec.backoffLimit` | `2` | 一時的な失敗(ネットワーク・DB の再起動)を2回まで再試行 |
| `podFailurePolicy` | 終了コード 2・3 は `FailJob` | 再試行しても同じ結果(§3)なので、無駄な再試行と取得元への再アクセスをしない。`restartPolicy: Never` が前提 |
| `activeDeadlineSeconds` | `3600` | 初回・版の変更時は Showdown の tarball 取得と `npm ci && node build` がある。これを超えたら異常 |
| `ttlSecondsAfterFinished` | `1209600`(14日) | 週1回の結果を次の実行より長く残す。`make import-k8s` で作った手動の Job も最終的に消える |

- Pod: `restartPolicy: Never`、`automountServiceAccountToken: false`、`runAsNonRoot`、`seccompProfile: RuntimeDefault`。
  コンテナ `import`: `allowPrivilegeEscalation: false`、`capabilities.drop: [ALL]`、`readOnlyRootFilesystem: true`、
  `imagePullPolicy: IfNotPresent`。イメージは `pokecalc/pokedex-importer:<semver>`(`latest` にしない)。
- DB の接続: 既存の Secret `mysql-auth` の `pokedex-dsn` を `POKEDEX_DATABASE_DSN` に `secretKeyRef` で渡す(値を manifest に書かない)。
  MySQL の起動待ちの initContainer は置かない(週1回の時点で MySQL は動いている前提。動いていなければ終了コード1で再試行、それでも駄目なら失敗)。
- ボリューム:

| マウント先 | 中身 | 種類 | 理由 |
|---|---|---|---|
| `/app/data/generated` | 取得のキャッシュ(`.cache/`)・スナップショット・報告(`reports/`)・上流の検出(`upstream/`) | PVC `pokedex-import-cache`(`ReadWriteOnce`、2Gi) | キャッシュがあれば固定版の再取得をしない(取得元への負荷を週0回に近づける。§6)。報告を Job の後も残す |
| `/app/data/local` | 日本語名の override(任意) | ConfigMap `pokedex-name-overrides`(`optional: true`、読み取り専用) | Git 管理外(§7)。無ければ空のディレクトリ = override なし(version `none`) |
| `/tmp` | npm のキャッシュ・HOME | emptyDir | ルートを読み取り専用にするため(`HOME=/tmp`、`npm_config_cache=/tmp/npm-cache`) |

- `data/importer/*.json`(config・effects・regulations)はイメージに焼く(§6)。ConfigMap にしない。
- PVC の削除は自動化しない(データの削除。CLAUDE.md の人間の確認事項)。キャッシュを捨てたいときは人が消す。

### 6. イメージ(`services/pokedex/Dockerfile` の `importer` ターゲット)

```
FROM golang:1.27.1-alpine@sha256:... AS build   # 既存。./pokedex/cmd/import も build する(/out/pokedex-import)
FROM node:<X.Y.Z>-alpine@sha256:... AS importer-deps  # tools/importer の package.json・package-lock.json だけを COPY して npm ci --omit=dev
FROM node:<X.Y.Z>-alpine@sha256:... AS importer
  /app/tools/importer/*.mjs・cronjob.sh・node_modules(deps から)
  /app/data/importer/config.json・effects.json・regulations.json
  /app/pokedex-import(build から)
  USER node(uid 1000)/ ENTRYPOINT ["/app/tools/importer/cronjob.sh"]
```

- ベースイメージは ADR-0102 のとおりタグ(完全な版)+ digest で固定する。Node は導入時点の最新の安定版(web の `engines.node` と
  同じ系列を推奨。確認した版と digest を ADR-0102 の流儀で本 ADR の末尾に追記する)。
- **設定をイメージに焼く理由(GitOps)**: kustomize は既定でキーの外(`data/importer/`)のファイルを ConfigMap にできず
  (load restrictor)、Argo CD にも同じ制約がかかる。deploy/ に写しを置くと版の二重管理になる。イメージは Git の commit から作るので、
  「その commit の固定版と設定」を中に持つのが一貫する。ローカルは `make up` が作り直す。GitOps の overlay(ADR-0018)で配るときは、
  版を上げた PR で新しいイメージの digest を overlay に書く(= Git の変更が版の変更になる)。
- **焼かないもの**: `data/generated/`(実データ)・`data/local/`(override。実データ)・`node_modules`(手元の物)。
  ルートに `.dockerignore` を置き、`data/generated`・`data/local`・`**/node_modules`・`.env` を除外する(第三者データをイメージに入れない。ADR-0002)。
- 実行時のネットワーク: calc はイメージの `node_modules` の固定版から抽出するので取得元へ出ない。Showdown の tarball・その `npm ci`・
  PokeAPI の CSV はキャッシュ(PVC)に無いときだけ取る。k3d の Pod から外向きの通信は既定で許されている(NetworkPolicy は v1 で置かない)。

### 7. override の渡し方と make import との関係

- `data/local/name_ja_overrides.json`(Git 管理外)がある場合だけ、`scripts/up.sh` が ConfigMap `pokedex-name-overrides`
  (キー `name_ja_overrides.json`)を作る/更新する(`kubectl create configmap ... --dry-run=client -o yaml | kubectl apply -f -`)。
  無ければ何もしない(既存の ConfigMap も消さない。消すのは人)。ConfigMap にするのは秘密ではないため(値は第三者の名前データで、
  認証情報ではない)。manifest には書かない(Git に入れない。Secret mysql-auth と同じく up.sh が作る)。
- **手動の `make import` と CronJob は同じ CLI・同じ入力の形**。手元と CronJob で config・override が同じなら checksum も同じになり、
  互いに「版が同じ」と判定して上書きし合わない。手元の未マージの config で同じ DB に `make import` すると、次の CronJob が
  Git の固定版に戻す(Git が正)。
- Makefile:
  - 既存: `import`(手元の data/ から投入)・`import-dry-run`・`import-fetch` はそのまま。
  - `import-check-upstream`(新規): `cd tools/importer && node check-upstream.mjs`(ネットワーク。3回)。
  - `import-k8s`(新規): kubectl の context が `k3d-$(CLUSTER)` であることを確かめてから
    `kubectl -n pokecalc create job --from=cronjob/pokedex-import pokedex-import-manual-<時刻>`(CronJob を手動で1回流す)。
  - `k8s-render`(新規): `kubectl kustomize deploy/k8s/overlays/local` と `.../cloud` を出力を捨てて実行(描画できることの確認)。
    `lint` から呼ぶ。`lint` は `tools/importer/*.sh` の構文(`sh -n`)も見る。
  - **ネットワークに触るもの・DB が要るものは `make test` に入れない**(ADR-0101 §11)。

### 8. up.sh と cloud overlay

- `scripts/up.sh`: migrate と同じ流儀で `docker build -f services/pokedex/Dockerfile --target importer -t "$POKEDEX_IMPORTER_IMAGE" .` と
  `k3d image import`(既定 `pokecalc/pokedex-importer:0.1.0`。CronJob の image と同じ値)。override の ConfigMap(§7)。
  **up.sh は CronJob を即時に流さない**(初回の取得はネットワークが要るため、人が `make import-k8s` で流す。完了メッセージで案内する)。
  CronJob は Job と違い spec を apply で更新できるので、migrate のような事前の delete は要らない。
- cloud overlay: CronJob・PVC は base に置く(pokedex の本番の構成要素。requirements の `importer (CronJob) ─▶ MySQL`)。
  ただし cloud には MySQL・`mysql-auth`・イメージの配布経路がまだ無い(ADR-0100 §9 の Job と同じ注意)ので、
  **cloud overlay は CronJob `pokedex-import` に `suspend: true` のパッチを当てる**(毎週失敗し続けないように)。cloud を使うときに外す。

### 9. 失敗の扱い

- 照合の Blocker(ADR-0103 §1)・`ErrKeyChanged`・入力の不正: 報告(`reports/latest*.json`・`latest-summary.txt` は PVC、要約は stdout)を
  残して終了コード3。`Apply` は呼ばれないので DB は変わらない。Job は `FailJob` で再試行しない。
- migrate が済んでいない DB(`data_versions` が無い): `NewSQLStore` の `AppliedVersions` が `ErrSchemaNotReady` を返し、何も書かずに終了コード3。
  テーブルを作ったりしない。
- `Apply` の途中の失敗は1トランザクションで全部戻る(ADR-0101 §9。既存)。

### 10. Go の API(テストが前提にする名前)

```go
// package importer(services/pokedex/importer)
var ErrSchemaNotReady error // data_versions が無い(migrate が済んでいない)

type Store interface {
    AppliedVersions(ctx context.Context) ([]SourceVersion, error)
    Apply(ctx context.Context, out Output, versions []SourceVersion, now time.Time) error
}
func NewSQLStore(db *sql.DB) Store // 既存の AppliedVersions / Apply を包む。MySQL 1146(テーブルが無い)は ErrSchemaNotReady で包む
func RunStore(ctx context.Context, s Store, out Output, versions []SourceVersion, now time.Time, force bool) (bool, error)
// 既存の Run(ctx, db, ...) は RunStore(ctx, NewSQLStore(db), ...) と同じ挙動にする(シグネチャは変えない)

type UpstreamLatest struct {
    SchemaVersion int               `json:"schemaVersion"`
    CheckedAt     string            `json:"checkedAt"` // RFC3339
    Sources       map[string]string `json:"sources"`
    Errors        map[string]string `json:"errors"`
}
func DecodeUpstreamLatest(raw []byte) (UpstreamLatest, error) // 厳格。失敗は ErrInvalidInput

type UpstreamState string
const (
    UpstreamSame    UpstreamState = "same"
    UpstreamDiffers UpstreamState = "differs"
    UpstreamUnknown UpstreamState = "unknown"
)
type UpstreamStatus struct{ Source, Pinned, Latest string; State UpstreamState; Detail string }
func CompareUpstream(pinned map[string]string, latest UpstreamLatest, now time.Time, maxAge time.Duration) []UpstreamStatus
func FormatUpstream(statuses []UpstreamStatus) string // 決定的。differs の行は "UPSTREAM" と source・固定版・上流の版・data/importer/config.json を含む

// package main(services/pokedex/cmd/import)
type cliEnv struct {
    Stdout, Stderr io.Writer
    Getenv    func(string) string
    OpenStore func(dsn string) (importer.Store, io.Closer, error)
    Now       func() time.Time
}
func run(args []string, env cliEnv) int // main は os.Exit(run(os.Args[1:], 本番の cliEnv))
```

- `RunStore`: `AppliedVersions` → `NeedsImport`(force なら常に)→ `Apply`。`AppliedVersions` が失敗したら `Apply` を呼ばずにそのエラーを返す。
  `NeedsImport` が `ErrInvalidInput` なら `Apply` を呼ばない。
- `CompareUpstream`: `pinned` の source の昇順。`latest` にだけある source は無視。`checkedAt` が解釈できない・`now - checkedAt > maxAge` なら
  全 source が `unknown`(Detail に `stale`)。`errors` にある source は `unknown`(Detail に理由)。検出結果に無い source は `unknown`
  (Detail に `not-checked`)。

## 受け入れ条件(テストの担当は §テスト)

1. CronJob `pokedex-import` が base/pokedex にあり、週1回(`分 時 * * 曜日` の形)・`timeZone: Asia/Tokyo`・`concurrencyPolicy: Forbid`・
   `startingDeadlineSeconds` が 0 より大きく 7 日未満・履歴の保持数 1〜5・`restartPolicy: Never`・`podFailurePolicy` で終了コード 2/3 を
   `FailJob`・`activeDeadlineSeconds`・`ttlSecondsAfterFinished` が 7 日以上、を持つ。
2. CronJob の DSN は Secret `mysql-auth` の `pokedex-dsn` から渡し、値入りの Secret・env の平文の DSN を持たない。コンテナは非 root・
   読み取り専用ルート・権限昇格なし。`/app/data/generated` は PVC `pokedex-import-cache`、`/app/data/local` は任意の ConfigMap
   `pokedex-name-overrides`、`-force` を渡さない。cloud overlay は `suspend: true` を当て、base は suspend しない。
3. イメージは `services/pokedex/Dockerfile` の `importer` ターゲットで、Go の `pokedex-import` と Node の取得スクリプトを含み、
   すべての外部ベースイメージが「完全な版のタグ+digest」。`data/importer` は焼き、`data/generated`・`data/local` は焼かない
   (`.dockerignore` でも除外)。CronJob の image と up.sh の既定値が一致し、up.sh が build・k3d import・override の ConfigMap を行う。
4. `cronjob.sh` は 取得 → 上流の検出(失敗は許容)→ `pokedex-import -upstream` の順で、`-force`・`-dry-run`・migrate を含まない。
5. `RunStore`: DB の版と同じなら `Apply` を呼ばない・違えば1回呼ぶ・force なら呼ぶ・版が読めない(migrate 未済を含む)なら呼ばない。
6. 上流の検出は報告だけ: `CompareUpstream` が same/differs/unknown(errors・未検出・古い)を返し、CLI は上流の版が違っても
   **固定版のまま投入し**(Apply に渡る版は config の版)、`config.json` を変えず、終了コードに影響しない。
7. CLI の終了コード: 照合の Blocker・`ErrSchemaNotReady`・`ErrKeyChanged` は 3 で DB を変えない(Blocker は DB を開きもしない)、
   DSN が無いと 2、dry-run は DB を開かない。
8. Makefile に `import-k8s`(context の確認つき `create job --from=cronjob/pokedex-import`)・`import-check-upstream`・`k8s-render`
   (local と cloud)があり、`lint` から `k8s-render` と `tools/importer/*.sh` の構文検査を呼ぶ。`make test` にネットワーク・DB を混ぜない。

## テスト

| AC | テスト(すべて `make test` = `go test ./...` in services。ネットワーク・DB なし。最後の1件だけ `make test-db`) |
|---|---|
| 1, 2 | `services/pokedex/importer/cronjob_layout_test.go`: `TestImportCronJobSchedule`・`TestImportCronJobJobSpec`・`TestImportCronJobPodSecurityAndWiring`・`TestImportCronJobInBaseAndSuspendedInCloud` |
| 3 | 同: `TestImporterImageDockerfile`・`TestDockerignoreExcludesRealData`・`TestUpScriptBuildsImporterAndOverrides` |
| 4 | 同: `TestCronJobScriptOrder` |
| 8 | 同: `TestMakefileImportTargets` |
| 5 | `services/pokedex/importer/store_test.go`: `TestRunStore*` |
| 6 | `services/pokedex/importer/upstream_test.go`: `TestDecodeUpstreamLatest*`・`TestCompareUpstream`・`TestFormatUpstream` / `services/pokedex/cmd/import/main_test.go`: `TestRunUpstreamDiffersIsReportOnly` 他 |
| 7 | `services/pokedex/cmd/import/main_test.go`: `TestRun*` |
| 9(migrate 未済) | `services/pokedex/importer/mysql_test.go`(`-tags mysql`、`make test-db`): `TestRunSchemaNotReady` |

- kustomize の描画は `make k8s-render`(`kubectl kustomize` の local / cloud)で確かめる。`go test` からは kubectl を呼ばない
  (静的検査は生の YAML を読む)。

## 限界

- 上流の検出は「違う」までで、新しい版に何が入ったか(裁定が要るか)は人が `make import-dry-run` で見る。
- 報告(`reports/import-<時刻>.json`)は毎週 PVC に1つ増える(1年で約50個)。掃除は v1 では行わない(容量が問題になったら保持数を足す)。
- PVC を失うと次の実行で固定版を取り直す(取得元へ1回ずつ)。k3d の local-path の PVC はノードのディスク上にあり、クラスタ削除で消える。
- ローカルの k3d はノート PC が止まっていれば動かない。`startingDeadlineSeconds` の範囲で1回だけ追いつく。
- cloud での実運用(マネージド DB・レジストリ・suspend の解除)は後続。

## 人間の確認事項(既定案で進める)

1. **上流に新しい版があっても Job は成功のまま(ログの `UPSTREAM` 行と報告だけ)**。既定: 成功のまま。
   代案: 失敗にして `kubectl get jobs` で目立たせる(人が PR を出すまで毎週失敗が続く)。
2. **スケジュールは土曜 12:00(日本時間)**。既定: `0 12 * * 6`。ノート PC の稼働時間に合わせて変えてよい(曜日と時刻だけの変更)。

## 実装時に確認した版(ADR-0102 の流儀。implementer が追記)

`services/pokedex/Dockerfile` の importer ステージ(importer-deps・importer)のベースイメージは
`node:26.9.0-alpine`(digest `sha256:dbaa92e5758cbbcf85d65d5403fdb530fe3442cbe8c6dbfb7ef23365450d5070`、
`docker pull` で確認。2026-09-22 時点)。`web/package.json` の `engines.node`(`26.9.0`)と同じ版に揃えた
(§6 の推奨どおり)。`docker build --target importer` で実際にビルドし、コンテナ内で
`sh -n tools/importer/cronjob.sh` の構文と `/app/pokedex-import -h` の起動、`USER node` での実行を確認した。

## 追記(issue #106 / ADR-0109。2026-09-23)

§5 の `concurrencyPolicy: Forbid` の理由欄「手動実行(`make import-k8s`)と重なっても同時に投入しない」は不正確
だった。Kubernetes の `concurrencyPolicy` は**同じ CronJob が作る Job 同士**にしか働かず、`make import-k8s`
(`kubectl create job --from=cronjob/...`)が作る独立した Job とは排他しない。実際の排他(`cronjob.sh` での
`flock`)は ADR-0109 を参照。`Forbid` 自体は「同じ CronJob の Job 同士の重複防止」という限定された役割のまま維持する。
