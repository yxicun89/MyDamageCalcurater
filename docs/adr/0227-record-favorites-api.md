# ADR-0227: お気に入り(手動ピン留め)の API 契約(record-svc。P5-3c)

- 状態: 採用(critic PASS。2026-10-04。2026-10-03 に提案・実装)
- 日付: 2026-10-03
- 関連: ADR-0209(保持・削除・端末 ID 境界。§3 #3・§4・§5・§6 がこの ADR の前提)、ADR-0213(team の CRUD。
  リソース設計・マスタ照合・上限・1件削除の扱いの前例)、ADR-0208(上限を契約に書く・新しい code を足さない)、
  ADR-0200(エラー語彙)、ADR-0802(サービス間で揃えたエラーの形)、ADR-0202(gateway のルーティング・CORS)、
  ADR-0220(失効ジョブ)、ADR-0212(計算イベント)、requirements.md §2「あれば便利: お気に入り(手動ピン留め)」・§6・§8、
  docs/plan.md P5-3c、docs/ai-shared/decisions/2026-10-03-210-ios-4-ios-api-web.md(iOS レーンからの依頼)

## 背景

`favorites` 表(`services/record/db/migrations/000005`)・保持期間(`max(devices.last_seen_at, 行.updated_at)` から540日)・
端末単位の全削除の件数(`RecordDeletionResult.deleted.favorites`)・失効ジョブ(ADR-0220)は実装済みだが、
作成・削除・一覧の API が無く、Web・iOS の画面も作れない。ADR-0209 は「record にお気に入りの CRUD を足すときに
AC-D2(他端末のリソース ID は 404)を実操作で検証する」と申し送っている。iOS レーンは 2026-10-03 に契約の追加を依頼した。

決めること: (1) 操作と経路・更新の有無・冪等性、(2) リソースの形(何を保存するか・型付きか opaque か)、
(3) 件数上限・ページング、(4) エラー、(5) 計算イベントとの関係と `last_seen_at`、(6) 端末分離・全削除・失効との整合。

## 決定

### 1. 操作は一覧・作成・削除の3つ。更新(PUT / PATCH)と1件取得は持たない

| 操作 | メソッドとパス | 成功 | 備考 |
|---|---|---|---|
| 一覧 | `GET /api/record/favorites`(`listFavorites`) | 200(`Favorite[]`) | `updatedAt` の降順、同時刻は `id` の降順。ページングなし |
| 作成 | `POST /api/record/favorites`(`createFavorite`) | 201(新規)/ 200(同じ内容が既存) | §2 の冪等性 |
| 削除 | `DELETE /api/record/favorites/{favoriteId}`(`deleteFavorite`) | 204 | 2回目・他端末の ID は 404 |

- **更新を持たない理由**: requirements.md §2 は「手動ピン留め」だけで名前の変更を求めていない。Web・iOS にも
  お気に入りの編集画面の要望は無い(iOS の依頼は「取得・追加・削除」)。ピンは「その時点の個体のスナップショット」
  (ADR-0209 §3 #3)なので、中身を変えたいときは「外して付け直す」が意味として正しい。更新を足すと、
  重複判定(§2)と「同じ内容への更新」の衝突の扱いが要る。後から足しても後方互換なので、要望が出てから追加する
  (そのときは `PUT` で丸ごと置換。ADR-0213 §2 と同じ形)。
- **1件取得を持たない理由**: 一覧が上限100件・1件数百バイトで小さく(§3)、画面は一覧から選ぶ。
  AC-D2 の実操作での検証は削除で足りる(他端末の ID を指す操作が1つあれば、端末で絞る規則を試せる)。
- 1件の削除は冪等にしない(2回目は 404)。ADR-0213 §2 と同じ理由。

### 2. リソースの形: `Individual` + 任意の `label`。型付きで受け、正規化した JSON を保存する

`FavoriteInput = { label?: string | null (0..30 コードポイント), individual: Individual }`。
`Favorite = { id: FavoriteId, label: string | null, individual: Individual, createdAt, updatedAt }`。

- **何を保存するか**: 計算 API の `Individual` そのもの。Web(`web/src/domain/requests.ts` の `buildIndividual`)・
  iOS(生成型 `Components.Schemas.Individual`)とも、計算の攻撃側・防御側に渡している型で、ピンを開いたらそのまま
  計算に使える(写像が要らない)。構築の `TeamMember`(技・ニックネームを持つ)にしない理由: お気に入りはダメージ計算の
  個体を呼び出すためのもので、構築は team-svc が持つ(同じ物を二重に持たない)。技は計算ごとに選ぶ。
  `ranks` / `status` も `Individual` の一部として保存する(「この状態で計算した」を再現できる。要らなければ画面が既定値に戻す)。
- **`label`**: 「HB特化」のような任意の名前。30 コードポイント(TeamMember.nickname の 24 と構築名の 50 の間。
  一覧の1行に収まる長さ)。空文字は null。利用者の自由入力なので**ログに出さない**(ADR-0209 §3)。
- **型付き(`Individual` を契約で定義)にし、opaque JSON にしない**。トレードオフ:
  - opaque の利点: クライアントが自由に形を変えられ、サーバーの変更が要らない。
  - opaque の欠点: サイズ・中身の上限が無く(1MiB まで何でも入る)、**個人情報や任意の文字列を入れる経路**を作る
    (ADR-0209 §3「お気に入りの中身はログに出さない」を守るだけでなく、そもそも入る物を限りたい)。Web と iOS が
    別々の形を保存すると互いに読めない(同じ端末 ID を共有しないので実害は小さいが、契約の生成型の恩恵が無くなる)。
  - 型付きの欠点: `Individual` が変わると保存済みの行の読み方に影響する。ただし `Individual` は計算 API の契約で、
    変更は後方互換(任意項目の追加)で行う方針なので、古い行も読める。
  - 結論: 型付き。契約に無いキーは `individual` の中も含めて 400 `unknown_field`(既存の DisallowUnknownFields と同じ)。
- **正規化と保存の形**: httpapi は `individual` に既定値を補う(`level` 50・`ranks` は6キーすべて〈省略は0〉・
  `status` `none`・`abilityId` / `itemId` / `teraType` は null と省略を同じ「未指定」としてキーごと省く)。
  そのうえで `{"label":…,"individual":{…}}` を**キー順の決まった JSON** にして `favorites.snapshot` に入れる。
  応答の `individual` もこの正規化後の形。端末 ID・セッション ID は snapshot に入れない(行の `device_id` 列だけ)。
- **検証(すべて 400 `invalid_input`。境界ちょうどは通す)**: SP 各 0..32・合計 66 以下、ランク -6..+6、レベル 50、
  `speciesKey` の形式(`^[0-9]{4}-[0-9]{3}$`)、`natureId` が空でない、`sp` と StatBlock の6キーの欠落、
  `individual` の欠落、`label` の長さ。列挙(`teraType` / `status`)の未知の値は 400 `invalid_enum`、壊れた JSON・
  空の本文は 400 `invalid_json`(team と同じ)。
- **マスタと照合しない**: `speciesKey` 等が実在するかは見ない(record-svc は pokedex-svc に依存しない。
  ADR-0213 §3 と同じ理由: マスタの障害が保存を止める・レギュレーション変更で保存済みのピンが開けなくなる)。
  メガシンカの持ち物の制約(`Individual.itemId` の説明)も計算時に calc-svc が見る。ADR-0209 の検証要求
  (分離・保持・全削除・ログ)はマスタ照合を求めておらず、この判断と矛盾しない。
- **`favoriteId`**: `favorites.id`(BIGINT AUTO_INCREMENT)を**10進の文字列**で出す(`^[1-9][0-9]{0,18}$`)。
  数値にしないのは、JavaScript の数値で精度を落とさない(TiDB の AUTO_INCREMENT はノードごとに大きく飛ぶ)ためと、
  形式違い・int64 を超える値を「この端末が持っていない ID」と同じ 404 `not_found` に揃えるため(team の `teamId` と同じ)。
  連番で推測できるが、端末で先に絞る(§6)ので他端末の行には届かない。
- **冪等性(同じ内容の二重作成)**: 同じ端末に同じ正規化済み snapshot(`label` を含む)の行があれば、作らずに
  その行の `updatedAt` を現在時刻に進めて **200** で返す(新規は 201)。二重送信・再試行で同じピンが2つにならない。
  `updatedAt` を進めるのは「もう一度ピン留めした」=利用者がその行に触った操作で、一覧の先頭に来るのが自然なため
  (失効の判定 `max(last_seen_at, updated_at)` も進む。ADR-0209 §4 の「行自体を最後に触った操作」と一致)。
  判定は DB で行う: migration 000006 で `favorites` に `snapshot_hash CHAR(64) NULL`(snapshot のバイト列の
  SHA-256 の16進小文字)を、000007 で `UNIQUE KEY (device_id, snapshot_hash)` を足す(TiDB は新しい列と
  その索引を1つの `ALTER TABLE` に書くと `column does not exist` で拒むため、2つの版に分けた。実装時に実 TiDB で確認)。同時に同じ要求が来ても一意制約で1行になり、
  負けた側は既存の行を返す。000005 は適用済みなので書き換えない。NULL を許すのは、API 以前にテストが直接入れた行
  (`snapshot_hash` なし)と共存させるため(MySQL/TiDB の UNIQUE は NULL を重複と見なさない)。
- **読めない行**: 保存済みの snapshot が `Individual` として読めないときは、一覧を 500 `internal` にする
  (黙って飛ばすと利用者のピンが消えたように見える。サーバー側の不備なので気づける方を選ぶ)。

### 3. 件数上限は1端末100件。ページングなし

- `store.MaxFavoritesPerDevice = 100`。上限に達した端末からの(重複でない)作成は 400 `invalid_input`(新しい code を
  足さない。ADR-0208 §2・ADR-0213 §2 と同じ)。同じ内容の再ピン留めは行が増えないので上限でも 200。
- 件数の確認と挿入は同じトランザクションで行い、端末ごとに直列化する(例: `devices` 行を `SELECT … FOR UPDATE`。
  `touchAndRun` が先に `devices` 行を作るので行は必ずある)。同時の作成でも上限をすり抜けない。
  重複確認・件数確認も `FOR UPDATE`(現在読み)にする。TiDB の通常の SELECT はトランザクション開始時点のスナップショットを読むため、
  `devices` 行をロックしても上限をすり抜けた(実測: `TestTiDBCreateFavoriteLimitUnderConcurrency` で 105 行)。後から `FOR UPDATE` を外さない。
  時刻は store の入口で `DATETIME(6)` の精度(マイクロ秒)に丸め、応答と一覧の値を一致させる。
- 契約の一覧・削除に 500 を書かない理由: 既存の `listFrequentOpponents` など record の操作と同じく、想定外の失敗(`internal`)は共通の Error 形で返す前提で、
  利用者が分岐する応答だけを列挙する。作成の 500 は本文の正規化で起きうる失敗として残した(揃える必要が出たら後方互換で足せる)。
- 一覧は全件(最大100件)。1件は正規化後でも 400〜500 バイト程度で、応答は最大でも約 50KB。上限とページングの
  両方は要らない(ADR-0213 と同じ)。契約の一覧の `maxItems: 100` に上限を書く(ADR-0208 の流儀)。
- 本文の上限は既存の 1MiB(`maxRequestBodyBytes`)のまま。形が契約で閉じているので実際の本文は 1KB 未満。

### 4. エラーは既存の語彙だけ

`invalid_json` / `unknown_field` / `invalid_enum` / `invalid_input`(範囲・形式・上限)/ `not_found`(持っていない ID・
形式違いの ID)/ `missing_header` / `store_unavailable`(503)/ `internal`(500)。新しい code は足さない
(ADR-0208 §2・ADR-0802)。`ErrorCode` の表の `invalid_input` の行に「record のお気に入り」を追記した。

### 5. 計算イベントとの関係と `last_seen_at`

- お気に入りは計算イベントではない。JetStream のイベントからピンを作らない(`events` パッケージはお気に入りの
  メソッドを呼ばない。テストで固定)。ピン留めは利用者の明示の操作だけ。
- 3つの操作はどれも「端末 ID を含む HTTP 要求」なので、既存の `touchAndRun` を通して `devices.last_seen_at` を
  24時間規則で更新する(ADR-0209 §4。「端末の利用」と数える)。順序は既存の2操作と同じで、ヘッダ・クエリ・本文の
  検証を通った要求は、その後 404(持っていない ID)・上限の 400 になるものも含めて更新する(要求が来た事実は端末が
  使われている証拠)。本文の検証で弾く要求(壊れた JSON・範囲外)で更新するかは問わない。

### 6. 端末分離・全削除・失効との整合

- **分離(ADR-0209 §6)**: store の3メソッドは deviceID を必ず受け、SQL は `WHERE device_id = ?` で先に絞る。
  他端末の `favoriteId` は「実在しない」と区別せず 404(403 にしない・本文に情報を出さない)。端末 ID はヘッダだけで、
  クエリ・本文の `deviceId` は 400 `unknown_field`。重複判定も端末の中だけ(別の端末の同じ内容は別の行)。
- **全削除**: 既存の `PurgeDevice` が `favorites` を消して数える(変更なし)。**墓石(`purged_at`)は作成を止めない**:
  墓石は JetStream から遅れて届く古い計算イベントの再出現を防ぐためのもので(ADR-0209 §7)、全削除の後に利用者が
  新しくピン留めしたものは利用者の新しい意思なので保存する。
- **失効**: 失効ジョブ(ADR-0220)の判定 `max(devices.last_seen_at, favorites.updated_at)` は変えない。
  `updated_at` は作成時と同じ内容の再ピン留め(§2)で進む。それ以外で `updated_at` を進める操作は無い(更新を持たない)。
  一覧の表示は `updated_at` を進めない(読むたびに書くと書き込みが増える。端末の利用は `last_seen_at` が拾う)。

### 7. gateway・CORS・クライアント

- gateway の変更は無い: `/api/record/*` は前方一致で record-svc へ届き(ADR-0209 §10)、CORS の許可メソッドは
  `GET, POST, PUT, DELETE, OPTIONS` で揃っている。回帰テストだけ足す。
- `api.ServerInterface` が3メソッド増えるので、calc-svc・pokedex-svc(`record_notfound.go`)と team-svc(`stubs.go`)に
  呼ばれない 404 スタブを足す(ADR-0209 §10-1 の機械的な追従。各サービスの振る舞いは変えない)。
- **Web レーンへの依頼**: `web/src/record/recordClient.ts` に `listFavorites()` / `createFavorite(input)` /
  `deleteFavorite(id)` を足し(経路は上の表。ヘッダは既存と同じ `X-Device-Id`・`X-Session-Id`、作成だけ
  `Content-Type: application/json`)、型は `components["schemas"]["Favorite" | "FavoriteInput"]`。
  計算画面で今の攻撃側・防御側の個体をピン留めし、一覧から選んで攻撃側・防御側に読み込む画面を作る。
  エラーの扱い: 400 `invalid_input`(上限100件・範囲)は message を表示、404 は一覧の再取得、503 `store_unavailable`・
  `upstream_unavailable` は「保存できない(計算はできる)」と表示(既存の record の扱いと同じ)。作成の 200 は
  「すでにピン留め済み(先頭に移動)」として扱う。
- **iOS レーンへの依頼**: 生成クライアントの `listFavorites` / `createFavorite` / `deleteFavorite` と
  `Components.Schemas.Favorite` / `FavoriteInput` を使う(`FavoriteInput.individual` は `Individual` そのもの)。
  計算履歴(生のイベント)の取得 API はこの ADR の範囲外(別タスク。plan.md に残す)。

## 受け入れ条件

ADR-0209 の既存 AC をお気に入りで満たすもの: **AC-D1**(端末 A のピンは端末 B の一覧に出ない)・**AC-D2**(端末 B が
端末 A の `favoriteId` を DELETE すると 404 `not_found`。record での実操作の検証はここが初めて)・**AC-D3**
(クエリ・本文の `deviceId` は 400 `unknown_field`)・**AC-P1/AC-P5**(全削除が API で作ったピンを消して数え、
他端末を消さない)・**AC-R2/AC-R2b**(失効の判定式は変えず、`updated_at` は作成・再ピン留めで進む)・**AC-R5**・**AC-L1**。

この ADR で新設するもの:

- **AC-F1** 作成は 201 で、`id`(FavoriteId の形式)・`createdAt`・`updatedAt` をサーバーが決める(作成直後は等しい)。
  任意項目は往復し、省いた項目は正規化した形で返る。`id` / `createdAt` / `updatedAt` を送ると 400 `unknown_field`。
- **AC-F2** マスタを引かずに判断できる検証は 400 `invalid_input`。境界ちょうど(SP 32・合計66・ランク ±6・ラベル30文字)は通る。
- **AC-F3** マスタに無い ID(形式は正しい)でも 201 で保存できる。
- **AC-F4** 同じ内容(label + 正規化した individual)の2回目は 200 で同じ `id`、行は増えず `updatedAt` が進む。
  同時に送っても1行。label が違えば別のピン。別の端末の同じ内容は別のピン。
- **AC-F5** 1端末100件まで(100件目は通り101件目は 400 `invalid_input`)。上限でも再ピン留めは 200。
  同時の作成でも上限をすり抜けない。別の端末は影響を受けない。
- **AC-F6** 一覧は `updatedAt` 降順・同時刻は `id` 降順。無ければ空配列(`null` にしない・404 にしない)。
  読めない行があれば 500 `internal`。
- **AC-F7** 削除は 204(本文なし)、2回目は 404。形式違い・int64 を超える ID も 404。
- **AC-F8** 3操作とも `TouchDevice` をヘッダの端末で呼ぶ。
- **AC-F9** DB に届かなければ3操作とも 503 `store_unavailable`(DB のエラー文を出さない)。
- **AC-F10** 全削除の後でも新しいピン留めは保存される。計算イベントの消費はお気に入りに触らない。
- **AC-F11** 更新(PUT / PATCH)は契約に無く、サーバーも受け付けない(行を変えない)。
- **AC-F12** Web・iOS の生成型で `FavoriteInput.individual` と `Favorite.individual` が `Individual` そのもの(包む型を挟まない)。

## 却下した案

- **snapshot を opaque JSON にする**: §2。中身とサイズを限れず、生成型の恩恵も無くなる。
- **`TeamMember` 相当(技・ニックネーム)を保存する**: 構築と二重に持つことになる。お気に入りは計算の個体の呼び出し。
- **更新(PUT)を持つ**: §1。要望が無く、重複判定との衝突の扱いが増える。後から後方互換で足せる。
- **重複を 409 にする**: 新しい code / ステータスを足すことになり(ADR-0208 §2)、クライアントは「すでにある」ものを
  取り直す往復が要る。既存を 200 で返す方が単純で、二重送信にも安全。
- **重複を許す(冪等にしない)**: 二重タップ・再試行で同じピンが並ぶ。上限100件を無駄に消費する。
- **重複判定をアプリ側の「探してから入れる」だけで行う**: 同時の要求ですり抜ける。一意制約で DB に保証させる。
- **`favoriteId` を整数で出す**: JavaScript の精度と、形式違いの 404 の扱い(生成ラッパが 400 を返してしまう)の2点で劣る。
- **墓石の後の作成を拒む**: 全削除の後に利用者が新しく付けたピンまで捨てることになる。墓石は計算イベント用。
- **一覧の表示で `updated_at` を進める**: 読むたびに書き込みが出る。端末の利用は `last_seen_at` が拾う。

## 未決事項

- `label` の上限 30 文字と、正規化で `ranks` / `status` を保存すること(画面がそれをどう使うか)は、Web・iOS の画面設計で
  不都合が出たら見直す(緩める方向は後方互換)。
- 計算履歴(生のイベント)の取得 API(iOS の依頼のもう半分)は別タスク(plan.md に追加)。
