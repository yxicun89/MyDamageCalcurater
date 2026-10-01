# ADR-0213: team-svc の構築 CRUD の契約とマスタ照合・Showdown 形式の担当分離(P5-4)

- 状態: 提案(2026-09-25。P5-4 の spec-writer 工程。実装と critic レビューはこの後)
- 日付: 2026-09-25
- 関連: ADR-0209(保持・削除・端末 ID 境界。§3 #4・§4・§5・§6・§10 がこの ADR の前提)、
  ADR-0211(TiDB のプロビジョニングと `TEAM_*` 環境変数)、ADR-0212(計算イベントのワイヤフォーマット)、
  ADR-0200(API 契約とエラー語彙)、ADR-0202(gateway のルーティングとヘッダ検証・CORS)、
  ADR-0208(契約に上限を書く前例)、ADR-0204(pokedex-svc の内部 API。マスタの正本の所在)、
  ADR-0501「P6-2c」(iOS の構築ビルダーのドメイン型。`TeamLimits` / `TeamValidator`)、
  docs/requirements.md §2(構築ビルダー)・§6(データモデル)・§8、docs/plan.md P5-4

## 背景

P5-4 は team-svc(構築の CRUD と端末単位の全削除)を作る。ADR-0209 は保持期間・削除・端末 ID による
分離を決めたが、**構築そのものの API の形**(リソース設計・`TeamMember` のスキーマ・どこまでを
team-svc が検証するか)は決めていない。ADR-0209 §5.3 も team の分は「未移動」のままで、
`DELETE /api/team/device-data` と `TeamDeletionResult` が `api/openapi.yaml` に無い。

P5-3(record-svc)は読み取り(集計)と全削除の2操作だけだったので、この種の判断が要らなかった。
team-svc は本格的な CRUD を持つ最初のユーザーデータのサービスであり、次を決める必要がある。

1. リソース設計(何を1リソースとし、どの操作を置くか。部分更新を持つか)
2. `TeamMember` のスキーマ(どの ID を持ち、どの制約を契約に書くか)
3. ID をマスタ(pokedex-svc)と照合するか
4. Showdown 形式のインポート / エクスポート(requirements.md §2)をサーバーで実装するか
5. 計算イベント購読で team-svc が何をするか(ADR-0209 §4 の「`last_seen_at` だけ」の実装の形)

## 決定

### 1. 契約は `api/openapi.yaml` に置く(ADR-0209 §5.3 から移す)

ADR-0209 §5.3 の team の分(`DELETE /api/team/device-data`・`TeamDeletionResult`・`team` タグ)を
`api/openapi.yaml` へ移し、この ADR で決める CRUD も同じ区切りで入れる。移した時点で
**正は `api/openapi.yaml`** になり、ADR-0209 §5.3 は「移動済み」に書き換える(coding-rules §2「単一の正」)。

ADR-0209 §10 が「今は入れない」とした3つの理由(他サービスに空メソッドが増える / gateway に
ルートが無い / クライアントに呼べない型が生える)は、P5-4 で**同じ変更の中で**解消する:
gateway の `/api/team/*` ルーティングを足し、calc-svc・pokedex-svc・record-svc には
`team_notfound.go`(呼ばれない 404 スタブ)を足す。

### 2. リソース設計: `teams` 1つ。丸ごと置換で、部分更新を持たない

| 操作 | メソッドとパス | 成功 | 備考 |
|---|---|---|---|
| 一覧 | `GET /api/team/teams` | 200(`Team` の配列) | 更新の新しい順。ページングなし |
| 作成 | `POST /api/team/teams` | 201(`Team`) | `id` / `createdAt` / `updatedAt` はサーバーが決める |
| 取得 | `GET /api/team/teams/{teamId}` | 200(`Team`) | 持っていない ID は 404 `not_found` |
| 更新 | `PUT /api/team/teams/{teamId}` | 200(`Team`) | 名前とメンバー**全体**を置き換える |
| 削除 | `DELETE /api/team/teams/{teamId}` | 204(本文なし) | 2回目は 404 |
| 全削除 | `DELETE /api/team/device-data` | 200(`TeamDeletionResult`) | ADR-0209 §5(冪等・`partial`) |

- **メンバー個別の PATCH(`/teams/{id}/members/{slot}`)を作らない**。理由: 構築は多くても6体で、
  画面(Web P5-5 / iOS P6-2c)はすでに「編集中の構築を1つ手元に持ち、保存で確定する」形になっている
  (iOS の `TeamEditViewModel` がそう。ADR-0501)。部分更新を足すと、並べ替え・同時編集・
  「members の一部だけ送ったときに残りをどうするか」という状態が API 側に増える。
  複雑さに対して得られるのは1回あたりの転送量の削減だけで、1構築は数 KB に満たない。
- 同じ理由で **`members` を省いた PUT は「メンバーなし」への置換**として扱う(部分更新と読まない)。
  `PUT` の意味を「送った内容がそのまま保存後の状態になる」に統一し、分岐を作らない。
- **メンバーの並び順は `members` 配列の順序**そのものとし、契約に `slot` を持たせない
  (順序の正が2つになるのを避ける)。DB の `team_members.slot` は保存時に配列の添字から決める。
- **`teamId` はサーバーが発行する UUID**。クライアントが ID を決められると、他端末の ID を狙って
  作る経路と、ID の衝突の扱いが生まれる。
- **1端末が持てる構築は 100 件**まで(超えたら 400 `invalid_input`)。理由: 個人利用で 100 構築は
  実用上の上限を十分に超えており、これがあることで一覧にページングが要らなくなる
  (契約に上限を書くのは ADR-0208 の前例どおり)。件数の確認と挿入は同じトランザクションで行う。
- **1件の削除は冪等にしない**(2回目は 404)。端末単位の全削除(ADR-0209 §5.2)が冪等なのは
  「端末 ID だけで決まる要求を再送させる」ためで、1件の削除には再送の必要がない。
  404 は「他端末の構築」と同じ応答で、存在の有無を漏らさない(ADR-0209 §6-2)。

### 3. `TeamMember` は ID をそのまま運ぶ。マスタとは照合しない

`TeamMember` = `speciesKey` / `nickname` / `moveIds`(最大4・重複不可)/ `itemId` / `abilityId` /
`natureId` / `sp`(`StatBlock`。`Individual.sp` と同じ形)/ `teraType`。
iOS の `TeamMember`(ADR-0501「P6-2c」)と同じ形にして、クライアント側の写像を素直に保つ。

**team-svc は `speciesKey` / `moveIds` / `itemId` / `abilityId` / `natureId` がマスタに実在するかを
検証しない。** 理由:

- team-svc は pokedex-svc の DB に触れない(CLAUDE.md 絶対ルール4)。照合するなら pokedex-svc の
  内部 API を同期で呼ぶことになり、**マスタが落ちると構築の保存もできなくなる**(絶対ルール5 の
  「保存の障害を計算に波及させない」と同じ考え方で、逆方向の依存も作らない)。
- ID の妥当性は2か所ですでに担保される: クライアントは pokedex-svc の検索 API
  (`searchSpecies` / `searchMoves` / `searchItems` / `listNatures` / `getSpecies` の `abilities`。
  いずれも実装済み)から選ばせ、計算時には calc-svc が `unknown_species` / `unknown_move` /
  `unknown_item` / `unknown_ability` / `unknown_nature` で拒否する(ADR-0200)。
- レギュレーションは変わりうる(M-C をロジックに直書きしない。CLAUDE.md ドメイン規約)。
  保存時に「今のレギュレーションで使える」を強制すると、レギュレーション変更で**保存済みの構築が
  開けなくなる**。保存は素通しにして、使えるかどうかは計算時と画面の表示で判断する。

一方、**マスタを引かなくても判断できることは検証する**(すべて 400 `invalid_input`):
構築名(前後の空白を除いて1〜50文字)・メンバー数(0〜6)・技の数(0〜4)と同一メンバー内の重複・
ニックネーム(0〜24文字)・SP(各 0〜32・合計 66 以下。CLAUDE.md ドメイン規約)。
文字数は Unicode コードポイントで数える(バイト数で数えると日本語で実質1/3になる)。
この一覧は iOS の `TeamValidator`(ADR-0501)と同じ規則で、上限だけを契約側に明示した。

`nickname` は利用者の自由入力で個人を特定しうるため、**DB には保存してよいがログには出さない**
(ADR-0209 §3。構築名も同じ)。

### 4. Showdown 形式の入出力はクライアント(Web / iOS)の担当。team-svc では実装しない

requirements.md §2 の「Showdown形式のインポート/エクスポート」は、**team-svc の API にしない**。

- 変換の本体は「表示名 ⇔ ID の解決」+ テキストの整形で、どちらも team-svc の DB を必要としない。
  名前の解決に要るのは pokedex-svc の検索 API で、クライアントはすでにそれを呼べる。
  team-svc に置くと、§3 で避けた「team-svc → pokedex-svc の同期依存」を持ち込むことになる。
- calc-svc が個体を ID で受け取り、名前の解決をクライアントに任せているのと同じ責務分担
  (ADR-0200)。team-svc も ID で構造化された `Team` / `TeamMember` の CRUD だけを提供する。
- インポートは「貼り付けたテキストを解釈し、解釈できない行を人に直させる」対話が要る機能で、
  画面のある側に置くほうが素直(サーバーに置くと、部分的な失敗を API のエラーで表現する必要が出る)。

反例の検討: 「Web と iOS で2回実装することになる」は事実で、これがサーバーに置く唯一の動機になる。
しかし両者は言語も UI も別で、共有できるのは変換規則(どの行が何を表すか)だけである。その規則は
テキストで共有すればよく、HTTP API にする理由にならない。**engine(WASM)に置く案**も検討したが、
変換にはマスタの日本語名・英語名が要り、engine は純粋(外部データを持たない。絶対ルール2)なので
入れられない。したがってクライアント側が正しい置き場所になる。

実装は Web(P5-5)・iOS(P6-2 で「後回し」とした分)の担当とし、plan.md にその旨を書く。将来サーバー側に置き直す
必要が出たら(例: Android を足して3回目の実装になるとき)、この判断を覆す ADR を書く。

### 5. 計算イベントの購読: `devices.last_seen_at` だけを更新する

ADR-0209 §4 のとおり、team-svc は P5-2 の `CALC_EVENTS` を**自分の durable consumer**
(`team-svc`。record-svc の `record-svc` とは別)で購読し、自分の DB の `devices.last_seen_at` を
更新する。イベントの `Detail`(個体・ダメージ)は**読まない・保存しない**。

- `store.Store` のイベント経路のメソッドは `TouchDeviceFromEvent(ctx, deviceID string, occurredAt time.Time)`
  だけで、**計算の中身を運べる引数を持たない**。これが「team-svc は計算の中身を保存しない」の
  実装上の保証になる(テストが署名を固定する)。
- **重複排除の表を持たない**。24時間規則(ADR-0209 §4: 直近の `last_seen_at` から24時間以内は書かない)が
  そのまま冪等性になるため、同じイベントが再配送されても2回目は必ず「書かない」で終わる
  (record-svc が `calc_events.event_id` の一意制約を必要としたのは、行を保存するからである)。
  `eventID` はログの手がかりとしてだけ受け取る。
- 墓石(ADR-0209 §7)は同じように効かせる: `occurredAt <= devices.purged_at` のイベントは
  `last_seen_at` を進めず、ack して捨てる。これが無いと、削除済みの端末の `devices` 行が
  遅れて届いた古いイベントで延命される。

### 6. 環境変数(ADR-0211 §7 に従う)

`TEAM_ADDR` / `TEAM_APP_DSN`(必須)/ `TEAM_NATS_URL`(空なら購読を無効化)/
`TEAM_RETENTION_DAYS`(540)/ `TEAM_DEVICE_ROW_EXPIRY_DAYS`(30。JetStream の `max_age` 7日より大きいこと)/
`TEAM_PURGE_JOURNAL_RETENTION_DAYS`(90)/ `TEAM_PURGE_BATCH_LIMIT`。
日数は未設定・0以下なら起動しない(既定値へのフォールバックをしない)。

### 7. gateway(ADR-0202 への追記)

`/api/team/*` → team-svc(`GATEWAY_TEAM_URL`。未設定なら 503 `upstream_unavailable`)。
`/api/team` そのもの・`/api/teamx` は 404。ヘッダ検証は `/api/*` と同じ。
CORS の `Access-Control-Allow-Methods` に **`PUT`** を足す(`DELETE` は P5-3 で追加済み。`PATCH` は
§2 で部分更新を持たないと決めたので足さない)。

## 受け入れ条件

ADR-0209 の既存 AC を team の文脈で満たすもの(再掲): **AC-D1**(端末 A の構築は端末 B の一覧に出ない)・
**AC-D2**(端末 B が端末 A の `teamId` を指した GET / PUT / DELETE は 404 `not_found`。403 にしない・
A の情報を漏らさない。**record-svc では試せる操作が無かったこの AC の実テストは team-svc が持つ**)・
**AC-D3**(ボディ・クエリの `deviceId` は 400 `unknown_field`)・**AC-D4**(gateway のヘッダ検証)・
**AC-P1 / AC-P1b / AC-P2 / AC-P5 / AC-P6 / AC-P7**(全削除の冪等・`purged_at` の再発行・`partial` の
繰り返し・他端末を消さない・部分障害で 503 と墓石・record DB を消さない)・**AC-P8**(CORS の DELETE)・
**AC-P9**(`/api/team/*` のルーティングと 503)・**AC-R2d**(イベント消費で `last_seen_at` が保たれる)・
**AC-R5**(24時間規則)・**AC-R7**(再配送で二重にならない)・**AC-R8**(起動時検証)・
**AC-L1**(ログに構築名・ニックネーム・個体の中身を出さない)・**AC-C1 / AC-C2**(契約)。

この ADR で新設するもの:

- **AC-T1** 作成は 201 で、`id`(UUID)・`createdAt`・`updatedAt` をサーバーが決める。作成直後は
  `createdAt == updatedAt`。要求に `id` / `createdAt` / `updatedAt` を入れたら 400 `unknown_field`。
- **AC-T2** マスタを引かずに判断できる入力検証(名前1〜50文字・メンバー0〜6体・技0〜4・技の重複・
  ニックネーム0〜24文字・SP 各0〜32・SP 合計66以下)は 400 `invalid_input`。**境界ちょうど
  (50文字・6体・4技・SP 32・合計66)は通る**(「以上で弾く」実装にしない)。
- **AC-T3** マスタに存在しない `speciesKey` / `moveIds` / `itemId` / `abilityId` / `natureId` でも
  201 で保存できる(team-svc はマスタを照合しない。§3)。`unknown_species` 等を返さない。
- **AC-T4** メンバーの任意項目(ニックネーム・持ち物・特性・テラスタイプ・技の並び)が
  作成 → 取得で往復する。空文字のニックネームは「未設定」として扱う。
- **AC-T5** 1端末が持てる構築は 100 件までで、超える作成は 400 `invalid_input`。上限は端末ごとで、
  別の端末は影響を受けない。
- **AC-T6** 一覧は更新の新しい順で、構築が無い端末は空配列(`null` にしない・404 にしない)。
- **AC-T7** `PUT` は名前とメンバー全体を置き換える(2体 → 1体にできる。`members` の省略は
  「メンバーなし」への置換)。`createdAt` は変わらず `updatedAt` は進む。
- **AC-T8** イベント消費で team-svc が store に渡すのは端末 ID と `occurredAt` だけで、CRUD の
  メソッドを呼ばない。`Detail` の中身は保存もログ出力もしない。
- **AC-T9** gateway の CORS プリフライトが `PUT` を許可する(これが無いと Web から構築を更新できない)。

## 却下した案

- **メンバー個別の PATCH を持つ**: §2。1構築6体・数 KB の規模に対して状態と分岐が増えるだけ。
- **`teamId` をクライアントが決める**: 他端末の ID を狙って作る経路と衝突の扱いが生まれる。
  端末 ID が分割キーでしかない(ADR-0209 §2)以上、ID の生成もサーバーに寄せるほうが単純。
- **保存時に pokedex-svc へマスタ照合する**: §3。マスタの障害が構築の保存を止め、
  レギュレーション変更で保存済みの構築が開けなくなる。
- **Showdown 形式の変換を team-svc に置く**: §4。DB を必要としない純粋なテキスト変換のために
  team-svc → pokedex-svc の同期依存を作ることになる。
- **Showdown 形式の変換を engine(WASM)に置く**: 変換にマスタの名前が要り、engine の純粋性
  (絶対ルール2)を壊す。
- **一覧をページングする**: 1端末 100 件の上限(§2)があるので不要。上限とページングの両方は要らない。
- **1件の削除を冪等(204 固定)にする**: 「消えていたのか、そもそも無かったのか」を利用者に
  返せなくなる。再送の必要が無い操作に冪等性は要らない。
- **team-svc でも `event_id` の重複排除表を持つ**: §5。行を保存しないので24時間規則で足りる。
  表を持つと、record-svc と同じ「ストリーム作り直しでシーケンスが巻き戻る」問題(ADR-0212 §4 の
  追記)を team-svc にも持ち込むことになる。

## 影響

- `api/openapi.yaml` に `team` タグ・5つの CRUD 操作・`deleteTeamDeviceData`・`TeamId` /
  `TeamMember` / `TeamInput` / `Team` / `TeamDeletionResult` が入る(`make gen` 済み)。
- ADR-0209 §5.3 の team の分は「移動済み(正は `api/openapi.yaml`)」になる。
- calc-svc・pokedex-svc・record-svc に `team_notfound.go`(404 スタブ)が増える(ADR-0209 §10-1)。
- gateway は `/api/team/*` のルーティングと CORS の `PUT` を持つ(ADR-0202 §3・§6 に追記)。
- Web(P5-5)・iOS(P6-2 で後回しにした分)が Showdown 形式の変換を担当する(plan.md に追記)。
- team-svc の失効ジョブ(ADR-0209 §4)と `deploy/k8s` への配線は、record-svc の P5-3b と同じく
  P5-4 の範囲外(plan.md の残作業に書く)。それまで `teams` / `team_members` は端末単位の
  全削除 API でしか消えない。

## 人間の確認が必要なこと

- **1端末 100 構築**の上限(§2)と**ニックネーム24文字**・**構築名50文字**(§3)は、この ADR の
  既定案で明示の了承はまだない。異論があれば数字だけを変える(契約と store の定数の2か所)。
- Showdown 形式をクライアント側の担当にした判断(§4)。requirements.md §2 は機能としてこれを
  求めているので、**機能自体をやめる判断ではない**(Web P5-5 と iOS レーンで実装する)ことを確認したい。
