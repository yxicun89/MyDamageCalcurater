# ADR-0206: calc・gateway を pokedex-svc につなぐ

- 状態: 提案(2026-09-22。受け入れ条件とテストは spec-writer が先に書き、実装は implementer。データレーンからの依頼)
- 日付: 2026-09-22
- 関連: ADR-0012 §6(サービス境界と実行時依存)、ADR-0100(pokedex のスキーマ)、ADR-0104(importer の CronJob と `make import`)、
  ADR-0105(pokedex-svc の公開 API・内部 API)、ADR-0200(calc-svc の契約)、ADR-0202(gateway のルーティング)、
  ADR-0203(k3d デプロイとスモーク。§5 に「pokedex-svc を入れたらここを 200 に変える」)、
  ADR-0204(calc-svc のマスタを pokedex-svc の内部 API から受け取る。§4・§5)、ADR-0205(Web を gateway の後ろに置く。
  スモークが「200 か 503」を許す前例)、CLAUDE.md 絶対ルール 1・4・5・6、docs/coding-rules.md §2(ハードコードしない)

## 背景

pokedex-svc(P2-3・ADR-0105)が main に入り、`deploy/k8s/base/pokedex` に Deployment と Service `pokedex`(80 番)がある。
一方、API レーンの2つのサービスはまだ pokedex-svc を見ていない:

- gateway: `GATEWAY_POKEDEX_URL` が未設定で `/api/pokedex/*` は 503 `upstream_unavailable`(ADR-0203 §3)。
- calc-svc: k3d ではファイル方式(`CALC_MASTER_PATH` + 例のマスタの ConfigMap。ADR-0204 §4)。

ADR-0203 §5 と ADR-0204 §5 はどちらも「pokedex-svc をデプロイしたら API レーンが切り替える」と書いて持ち越していた。
データレーンからの依頼は (1) gateway に `GATEWAY_POKEDEX_URL=http://pokedex` (2) calc を `CALC_MASTER_URL=http://pokedex`
(3) スモークの `/api/pokedex` を 503 → 200。

切り替えには、依頼に書かれていない結果が2つある。設計で扱う。

1. **どの overlay に置くか**: Component `deploy/k8s/overlays/local/api` は共有の `local` overlay と API レーン専用の
   `local-api` overlay の両方が読む(ADR-0203「apply の分離」)。`local-api` は pokedex-svc も MySQL も含まない。
2. **スモークの架空 ID が使えなくなる**: `services/gateway/scripts/smoke.sh` は例のマスタの架空 ID(`9001-000`・`testbeam`・
   `testneutrala`)で calc・bulk・reverse を叩く。calc-svc のマスタが pokedex の DB に変わると、その ID は存在しない
   (400/404 になりスモークが落ちる)。実データの ID をスクリプトに直書きすることは、マスタをコードに埋め込まない規約
   (CLAUDE.md ドメイン規約・coding-rules §2)とレギュレーション依存(M-C)の両方に反する。

## 決定

### 1. 設定は base に置き、overlay ごとに変えない

| ファイル | 変更 |
|---|---|
| `deploy/k8s/base/gateway/deployment.yaml` | env に `GATEWAY_POKEDEX_URL=http://pokedex` を追加(`GATEWAY_CALC_URL=http://calc` と同じ扱い。Service 名はどの環境でも同じ)。`GATEWAY_ASSETS_URL` は未設定のまま |
| `deploy/k8s/base/calc/deployment.yaml` | env に `CALC_MASTER_URL=http://pokedex` を追加。`CALC_MASTER_PATH`・`CALC_TYPECHART_PATH` は持たない |
| `deploy/k8s/overlays/local/api/calc-patch.yaml` | **削除**(calc への patch が要らなくなる) |
| `deploy/k8s/overlays/local/api/master.example.json` | **削除**(Component の下のコピー。元ファイル `services/calc/testdata/master.example.json` は残す) |
| `deploy/k8s/overlays/local/api/kustomization.yaml` | `configMapGenerator`(`calc-master-example`)と `patches` の `calc-patch.yaml` を削除。残るのは gateway の patch(CORS・`GATEWAY_WEB_URL`)と `images` |
| `deploy/k8s/overlays/local-api/kustomization.yaml` | **変更しない**(base から両方の設定を継ぐので `local` と同じ構成になる) |

base に置く理由:

- `pokedex` は base の resources に含まれる Service で、cloud overlay(`../../base` + CronJob の suspend patch)にも入る。
  `GATEWAY_CALC_URL=http://calc` を base に置いているのと同じ理由(Service 名は環境で変わらない)がそのまま当てはまる。
- ADR-0203 が「calc は base でマスタの場所を持たない」とした理由は「local 専用の ConfigMap を base から参照すると
  クラウドの overlay が壊れる」だった。URL 方式は ConfigMap を参照しないので、この理由は無くなる。
- **`local` と `local-api` が同じ Deployment を別の内容で描画しない**。両方とも namespace `pokecalc` の Deployment
  `calc` / `gateway` を作るので、内容が食い違うと「最後に apply した人が勝つ」状態になり、Argo CD(main の
  `deploy/k8s/overlays/local` を見ている)と `make api-k3d-deploy` が互いを上書きし続ける。`make api-smoke` の意味も
  「直前に誰が apply したか」に依存してしまう。

### 2. API レーン単体の確認は `make dev` と Go のテストで行う(k3d は pokedex-svc のあるクラスタを前提にする)

URL 方式の calc-svc は、マスタを取れるまで `/readyz` が 503 のままで Ready にならない(ADR-0204 §3)。したがって
pokedex-svc が居ないクラスタに `local-api` overlay だけを apply すると、calc の rollout は完了せず calc の API も 503 になる。
この ADR はそれを受け入れ、API レーンの「他レーンに依存しない確認」を次の2つに置く:

- `make dev`(k8s を使わない。`CALC_MASTER_PATH` の**ファイル方式**のまま。`scripts/dev.sh` は変えない)。
- Go のテスト(`calctest` が例のマスタで calc-svc の実物を起動する。契約テスト・スモークスクリプトのテストもこれを使う)。

k3d での確認(`make api-k3d-deploy && make api-smoke`)は、`make up`(pokedex-svc と MySQL を含む)と **`make import-k8s`
(初回のデータ投入)を済ませたクラスタ**を前提にする。`make up` は DB を作るだけで投入はしない(scripts/up.sh の最後の
案内・ADR-0104 の CronJob は週1)。未投入のクラスタでは pokedex-svc が 503 `master_unavailable` を返し、calc は Ready に
ならない。スモークはこの状態を見分けて、`make import-k8s` が要ることを出力する(§4)。

### 3. スモークは「マスタの入手元」を見分け、計算に使う ID をそこから取る

`services/gateway/scripts/smoke.sh` の計算の3ステップ(calc・bulk・reverse)で使う ID を、スクリプトの先頭で決める。

1. `GET /api/pokedex/natures` を最初に叩く(pokedex の確認と入手元の判別を兼ねる)。
2. **200 のとき(pokedex-svc に繋がっている)**: 公開 API から ID を取る。gateway 経由で叩くので端末ID・セッションIDは付ける。
   - 性格: `/api/pokedex/natures` の応答から**無補正**の性格(`"plus":"…"` を持たない要素。生成型 `Nature` の
     `plus` / `minus` は `omitempty` なので、無補正はキーごと落ちる)の最初の `id`。
     無補正を選ぶのは、逆算(reverse)が探索する性格クラスが neutral / plus だけで、下降補正を探索しないため(ADR-0010 §R3)。
   - 種族: `GET /api/pokedex/species?limit=1` の最初の `key`(攻撃側・防御側の両方に使う)。
   - 技: `GET /api/pokedex/moves?limit=200` のうち `"category":"physical"` で威力が 0 でないものの `id` を先頭から最大
     **10 件**。先頭から順に `POST /api/calc` を試し、200 かつ `maxDamage >= 1` になった最初の技を採用する
     (タイプ相性で無効化される組み合わせ[例: ゴースト/じめん など]を避ける。相性表はデータなので、スクリプトは
     引き当てを繰り返すことで避ける)。10 件すべて駄目なら失敗する。
3. **503 のとき(pokedex-svc に繋がっていない)**: 従来どおり例のマスタ(`services/calc/testdata/master.example.json`)の
   架空 ID を使う(`make dev`・pokedex-svc の無いクラスタ)。この架空 ID はスクリプトの先頭の変数のままにする。
4. 逆算の観測は、採用した技で `POST /api/calc` が返した `maxDamage` を `observations: [{"damage": …}]` に入れる
   (架空 ID のときの固定値 `{"percent":18}` をやめる)。同じ攻撃側・同じ技・同じ防御側で観測した実点数なので、
   真の構成(その種族・無補正・持ち物なし・その SP)が必ず候補に入り、`candidates` が空にならない。
5. `POST /api/calc` の期待は「200・`rolls` が16個・`category` が `physical`」のまま変えない(候補を物理技に絞るため)。
6. **使った入手元と ID を独立した1行で出す**(人が後から何で確かめたか分かるように。テストもこれを見る):
   `api smoke: master=pokedex species=<key> move=<id> nature=<id>` / `master=example …`。
   既存の最後の行(`api smoke: calc=200 bulk=200 … web=…`)は**文言も項目も変えない**(既存のテストの期待文字列を保つ)。
7. ID を引けずに終わるときの失敗メッセージには、**引けなかったエンドポイントのパス**(`/api/pokedex/natures`・
   `/api/pokedex/species`・`/api/pokedex/moves`)か、試して駄目だった操作(`/api/calc`)を含める。

例のマスタの架空 ID を残すのは、fixture の ID であってマスタの一覧ではないため(既存の扱いを変えない)。実データの ID は
1つもスクリプトに書かない。

実装の注意(スモークは POSIX sh + curl のまま。jq を足さない。coding-rules §4「シェル」):

- `grep -o` は POSIX に無い。配列の応答は `tr -d ' \n'` で1行にしてから `sed 's/},{/}<改行>{/g'` で要素ごとの行に割り、
  行ごとに `sed -n 's/.*"id":"\([^"]*\)".*/\1/p'` で取り出す(1行の中ではキーが1回しか出ないので貪欲一致でも安全)。
- `"power":0` の除外は `[,}]` まで見る(`"power":0,` と `"power":100,` を取り違えない)。
- `set -eu` の下で `… | head -1` は、パイプの左が SIGPIPE で落ちても全体が失敗しないように書く(コマンド置換で受ける)。

### 4. スモークの `/api/pokedex` の期待値(ADR-0205 の web と同じ形)

| 応答 | 扱い | 出力 |
|---|---|---|
| 200 | 成功。以降の ID は pokedex から取る(§3) | `pokedex=200` |
| 503 `upstream_unavailable` | 成功。pokedex-svc に繋がっていない構成(`make dev` 等)として扱い、架空 ID を使う | `pokedex=503` |
| 503 `master_unavailable` | 成功(この行だけ)。pokedex-svc には届いているが DB が未投入。stderr に「`make import-k8s` が要る」と出し、以降の失敗メッセージにもその注意を付ける | `pokedex=503` |
| それ以外(404・500・未知の code の 503) | 失敗 | ― |

- 503 を許しても検査は空振りしない。**calc-svc が URL 方式で動いている構成では、pokedex が落ちていれば calc も Ready に
  ならず、計算の3ステップが必ず失敗する**(gateway は endpoints の無い Service に届かず 503)。つまり「pokedex が 503 でも
  スモークが緑になる」のは、計算が別の入手元(ファイル方式)で成立している構成だけに限られる。
- 再試行の方針(ADR-0203 §5・ADR-0205)は変えない: 再試行するのは `000` と `502` だけ。`503` は意味のある最終状態なので
  再試行しない。
- 出力の最後の行は `pokedex=` の項目名も値の形(`200` / `503`)も変えない(既存のテストの期待文字列を保つ)。

### 5. 既存テストの移行(消さない・弱めない。CLAUDE.md 絶対ルール6)

| 既存 | 移行先 | 理由 |
|---|---|---|
| `cmd/calc.TestManifestCalcBaseHasNoLocalData` | `TestManifestCalcBaseUsesPokedexMaster` | 「base が local 専用のファイル/ConfigMap を参照しない」という中身の検査(`CALC_MASTER_PATH`・`CALC_TYPECHART_PATH`・volumes が無い)はそのまま残し、`CALC_MASTER_URL` についての期待だけを「無いこと」→「`http://pokedex` であること」に変える |
| `cmd/calc.TestManifestCalcLocalDataFromOverlayCopies` | `TestManifestCalcLocalUsesPokedexMaster` と `TestManifestCalcLocalHasNoMasterCopy` | 検査の対象(Component の下の例のマスタのコピー)が設計変更で無くなる。ADR-0204 が `typechart.example.json` を廃止したときと同じ形で、「新しい構成の検査」+「古いコピー・ConfigMap が残っていないことの検査」に置き換える。例のマスタが実際に読めることは `cmd/calc.TestNewHandlerServesExampleMaster` と `master.TestExampleExportLoads` が引き続き持つ(`make dev` とテストで使うため検査は消えない) |
| `cmd/gateway.TestManifestGatewayLocalConfig` | 同名のまま期待値を変更 | `GATEWAY_POKEDEX_URL` が「未設定であること」→「`http://pokedex`(Service の 80 番・パスなし)であること」。`GATEWAY_ASSETS_URL` 未設定・CORS の検査は残す |
| `deploytest.TestSmokeScriptFailsOnBrokenStack` の行「pokedex が答える(503 の確認が効いている)」 | `TestSmokeScriptFailsOnUnusablePokedexData` の表 | 200 が失敗ではなくなるので、「pokedex が 200 でも中身が使えない(無補正の性格が無い・威力のある物理技が無い・種族が空・ダメージが 0 の技しか無い)なら失敗する」に置き換える。空振り防止という役目は同じ |
| `deploytest.TestSmokeScriptPassesAgainstGatewayAndCalc` ほか既存のスモークのテスト | そのまま | pokedex 未設定(`503`)の構成なので期待文字列は変わらない。再試行の回帰テストも変えない |

`deploytest.MountedConfigMapFile` と `ConfigMapGenerator.SourceFor` は、移行後にどのテストからも使われなくなる。
使われない汎用機構を残さない(coding-rules §3)ので実装時に削除する(`ConfigMapGenerator` 型そのものは
`TestManifestCalcLocalHasNoMasterCopy` / `TestManifestCalcLocalHasNoTypeChartCopy` が使うので残す)。

### 6. 文書

- `services/calc/README.md`: k3d(base / local / local-api)は **URL 方式**(`CALC_MASTER_URL=http://pokedex`)、ファイル方式は
  `make dev` とテスト用、と書き換える。例のマスタの説明の「k3d local overlay」を外す。初回は `make import-k8s` が要ることを書く。
- `services/gateway/README.md`: 環境変数の表の `GATEWAY_POKEDEX_URL` の行に base の既定(`http://pokedex`)を書く。
  「k3d で動かす」の `/api/pokedex/*` は 503 という記述を、投入済みなら 200・未投入なら 503 `master_unavailable`
  (`make import-k8s`)に書き換える。
- `api/openapi.yaml` は**変えない**。pokedex の5操作の 503 は `upstream_unavailable` / `master_unavailable` の両方を
  すでに書いてあり、200 も定義済み(ADR-0105・ADR-0202 §8)。したがって `make gen` も不要。

## 受け入れ条件と担当テスト

| AC | 内容 | テスト |
|---|---|---|
| AC-P1 | base の gateway は `GATEWAY_POKEDEX_URL=http://pokedex`(Service 名・80 番・パスとクエリなし)を持ち、base の環境変数だけで `loadConfig` が通る。`GATEWAY_ASSETS_URL` は未設定、CORS は base に無い | `cmd/gateway.TestManifestGatewayBaseConfig` |
| AC-P2 | local overlay の Component を合成しても gateway の pokedex の設定は残る(CORS・`GATEWAY_WEB_URL` の patch が消さない)。calc は `http://calc`、assets は未設定、CORS は Vite の既定オリジンだけ | `cmd/gateway.TestManifestGatewayLocalConfig` |
| AC-P3 | base の calc は `CALC_MASTER_URL=http://pokedex` を持ち、`CALC_MASTER_PATH`・廃止した `CALC_TYPECHART_PATH`・volumes を持たず、base の環境変数だけで `loadConfig` が通る | `cmd/calc.TestManifestCalcBaseUsesPokedexMaster` |
| AC-P4 | local overlay の calc も同じ(Component は calc に patch を当てない)。Component に ConfigMap `calc-master-example` の generator が無く、`master.example.json` のコピーも残っていない。Deployment は ConfigMap を1つもマウントしない。元ファイル `services/calc/testdata/master.example.json` は残っている(`make dev`・テストが使う) | `cmd/calc.TestManifestCalcLocalUsesPokedexMaster` / `TestManifestCalcLocalHasNoMasterCopy` |
| AC-P5 | `local-api` overlay は従来どおり(namespace `pokecalc`、resources は `../../base/calc`・`../../base/gateway` だけ、components は `../local/api` だけ)。したがって `local` と同じ内容の calc・gateway を描画する | `deploytest.TestLocalAPIOverlayIsScopedToAPIServices`(既存)/ `TestLocalOverlayUsesAPIComponent`(既存) |
| AC-P6 | スモーク: pokedex が 200 を返す構成では、計算に使う ID(無補正の性格・種族・威力のある物理技)を `/api/pokedex/*` から**実際に**取り(natures・species・moves を叩き、出力の `master=pokedex species=… move=… nature=…` が引いた値と一致する)、calc・bulk・reverse が 200 になる(`pokedex=200`)。候補の先頭の技が計算するとダメージ 0 なら次の候補を試す。pokedex が未設定の構成では従来の架空 ID で成功する(`pokedex=503`・`master=example`) | `deploytest.TestSmokeScriptDiscoversIdsFromPokedex` / `TestSmokeScriptTriesAnotherMoveWhenTheFirstDealsNoDamage` / `TestSmokeScriptPassesAgainstGatewayAndCalc`(既存) |
| AC-P7 | スモーク: `/api/pokedex/natures` の 200・503 `upstream_unavailable`・503 `master_unavailable` は成功(最後の行は `pokedex=200` / `pokedex=503`)、`master_unavailable` のときは stderr に `make import-k8s` の注意が出る。404・500・未知の code の 503 は失敗する | `deploytest.TestSmokeScriptPokedexStates` |
| AC-P8 | スモークが空振りしない: pokedex が 200 でも、無補正の性格が無い・威力のある物理技が無い・種族が空・どの技でもダメージが 0(相性で無効)なら失敗する | `deploytest.TestSmokeScriptFailsOnUnusablePokedexData` |
| AC-P9 | 再試行は `000` / `502` だけのまま(pokedex の 503 を再試行で消さない)。壊れた構成では従来どおり失敗する | `deploytest.TestSmokeScriptRetriesThroughTransientBadGateway`(既存)/ `TestSmokeScriptFailsOnPersistentBadGateway`(既存)/ `TestSmokeScriptFailsOnBrokenStack`(既存・pokedex の行を移行) |
| AC-P10 | 文書: gateway の README の `GATEWAY_POKEDEX_URL` の行が base の既定 `http://pokedex` を書き、calc の README が k3d は URL 方式・ファイル方式は `make dev`/テスト用と書き、どちらかが初回の `make import-k8s` に触れている。ADR-0206 がある | `deploytest.TestPokedexWiringIsDocumented` |
| AC-P11 | `api/openapi.yaml` と生成物を変えない。`make test`・`make lint`・`make api-kustomize` が成功する | 手動 + `git diff --stat` |

## 却下した案

- **(a) `local-api` 専用に別の Component(ファイル方式)を新設する**: `local` と `local-api` が同じ namespace の同じ
  Deployment を別内容で描画することになり、`make api-k3d-deploy` と Argo CD(main の `local` を見ている)が互いを
  上書きし合う。`make api-smoke` の結果が「直前に誰が apply したか」に依存し、何を確かめたのか分からなくなる。
- **(b) 共有 Component はファイル方式のまま、`local` overlay だけに向き先を変える patch を足す**: 2つの env
  (`CALC_MASTER_PATH` と `CALC_MASTER_URL`)が同時に付き、calc-svc は「ちょうど1つ」でないと起動しない(ADR-0204 §3)。
  消すには strategic merge の `$patch: delete` か JSON 6902 patch が要り、`deploytest` の静的検査(ADR-0203 §3 が
  patch の書き方を env の上書き・volumes の追加に限っている)の範囲を超える。(a) と同じ二重定義の問題も残る。
- **(c) `local-api` overlay に pokedex-svc と MySQL も入れる**: ADR-0203 が `local-api` を作った理由(pokedex-migrate Job の
  再実行と mysql の上書きを避ける)を無くしてしまう。他レーンのリソースの所有権も壊す。
- **(d) gateway だけ繋ぎ、calc はファイル方式のまま残す**: 依頼の半分しか満たさず、ADR-0204 の本体(calc ← pokedex の
  内部 API)がクラスタで一度も動かないままになる。切り替えの手間を後ろにずらすだけ。
- **(e) スモークに実データの ID(`0025-000`・`thunderbolt` など)を書く**: マスタとレギュレーション(M-C)をコードに
  埋め込まない規約に反する(CLAUDE.md ドメイン規約・coding-rules §2)。使用可能集合が変わると壊れる。
- **(f) スモークの pokedex の期待値を 200 に固定する(`make dev` 用に環境変数で緩める)**: `make dev`・pokedex-svc が
  未デプロイのクラスタで必ず落ちる。ADR-0205 が web で「200 か 503」を選んだ前例と揃えない理由が無い。使われない
  設定項目(`API_SMOKE_POKEDEX`)を増やすことにもなる。
- **(g) スモークが `/internal/pokedex/master` を読んで ID を決める**: gateway は `/internal/*` を外に出さない(ADR-0204)。
  クラスタの外から叩ける口を増やさない。

## 影響

- **`make up` の直後(データ未投入)は calc-svc が Ready にならない**。k3d のローカル環境を使う全レーン(Web の画面の
  計算・iOS の実機確認)が、初回に `make import-k8s` を1回流す必要がある。スモークはその状態を見分けて案内を出す。
- `make api-k3d-deploy` は pokedex-svc が居るクラスタを前提にする(`make up` 済み)。k8s を使わない開発ループ
  (`make dev`)はファイル方式のままなので、API レーン単体で止まらずに作業できる。
- スモークが見るのは「calc と gateway が同じマスタの入手元を向いている」構成に限る。pokedex は繋がっているのに calc が
  ファイル方式、という混在は描画上あり得なくなる(設定が base の1か所だけになるため)。
- 例のマスタ(`services/calc/testdata/master.example.json`)は残り、`make dev`・`calctest`・契約テスト・スモークの
  fallback で使い続ける。Kustomize の Component の下のコピーだけが消える(コピーの同期を保つ手間も消える)。
- ADR-0203 §5 の「pokedex-svc を入れたら 200 に変える」と ADR-0204 §5 の「local overlay を URL 方式に切り替える」は、
  この ADR で実施済みになる。
