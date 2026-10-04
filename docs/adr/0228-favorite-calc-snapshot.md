# ADR-0228: お気に入りに計算の入力全体(`calc`)を保存し、選んだらすぐ計算できるようにする(F-09)

- 状態: 採用(critic PASS。2026-10-04)
- 日付: 2026-10-04
- レーン: API(record-svc の契約)。依頼元はダメージ計算(ec)レーンの取りまとめ
- 関連: ADR-0227(お気に入りの API。この ADR はその §2「リソースの形」を後方互換に広げる)、ADR-0209(保持・削除・端末 ID 境界・
  ログ)、ADR-0200(計算 API の契約とエラー語彙)、ADR-0208(上限を契約に書く・新しい code を足さない)、ADR-0213 §3
  (マスタと照合しない理由)、docs/usability-round2.md F-09、docs/ai-shared/decisions/2026-10-04-api-favorite-calc-team-name.md
  (Web・iOS への依頼)

## 背景

利用者の動作確認(usability-round2 F-09)の要望: 「お気に入りを登録しただけで何もできないなら無駄。クリックすると
そのときのダメージ計算がすぐ出せる」。ADR-0227 のお気に入りは `individual`(個体1体)と任意の `label` しか保存せず、
相手・技・場・急所・形式が失われる。Web・iOS は「お気に入りを開いたら計算結果が出る」を作れない。

制約:

- 既存クライアント(Web のお気に入り画面・iOS のお気に入りの読み込み)は `individual`+`label` だけを送り・読む。
  契約の生成型が壊れないこと(Swift の生成デコーダは必須キーの欠落で一覧全体の読み込みを失敗させる)。
- 保存済みの行の `snapshot` と `snapshot_hash`(重複判定。ADR-0227 §2)を変えないこと。migration を足さないこと。
- record-svc は calc-svc・pokedex-svc に依存しない(ADR-0227 §2・ADR-0213 §3)。

## 決定

### 1. `FavoriteInput`・`Favorite` に省略可の `calc`(`CalcRequest` への `$ref`)を足す。`individual` は必須のまま

- `calc` は `POST /api/calc` の本文 `CalcRequest` と同じ形で、契約では `$ref` だけで参照する(Go・TS・Swift の生成型が
  `CalcRequest` そのものになり、クライアントは保存した値をそのまま計算 API に渡せる。写像のコードが要らない)。
- **`individual` は必須のまま**にする。検討した2案:

  | 案 | 利点 | 欠点 |
  |---|---|---|
  | A: `individual` 必須のまま、`calc` を任意で足す(採用) | 契約の変更が「任意項目の追加」だけ。旧クライアントは何も変えずに動き、新しいお気に入り(`calc` 付き)も旧クライアントの一覧で読める。必須の組み合わせ条件が無く、生成型も素直 | 新クライアントは `calc.attacker` と同じ個体を `individual` にも入れる(約 300 バイトの重複) |
  | B: `individual` を任意にし「`individual` か `calc` のどちらか必須」 | 本文の重複が無い | OpenAPI 3.0 で「どちらか必須」を素直に書けない(`anyOf` は生成型を崩す)。応答の `individual` を任意にすると旧クライアント(Swift)が一覧全体を読めなくなるので、応答側はサーバーが `calc.attacker` から補う必要があり、入力と応答で規則が非対称になる |

  重複は数百バイトで上限(§3)に十分収まるため、単純さと互換を取って A にした。
- **`individual` と `calc.attacker` の一致は求めない**。`individual` は「一覧に表示する個体」で、通常は攻撃側だが、
  防御側を代表にしたい画面もありうる。一致を強制すると、旧クライアントが作った行(`calc` 無し)と規則が分かれるだけで得る物が無い。

### 2. 正規化と snapshot の正規形(既存行のハッシュを変えない)

- `calc` の `attacker` / `defender` は `individual` と**同じ正規化**(level 50・ranks 5キー・status none・未指定の
  abilityId / itemId / teraType はキーごと省く)。`format` は必須(欠落・未知は calc-svc と同じ 400 `invalid_enum`)、
  `moveId` は空でない文字列、`field` は `weather`・`terrain`・`attackerScreens`・`defenderScreens`(各 `reflect`・
  `lightScreen`・`auroraVeil`)を、`options` は `critical` を、すべて既定値(none・false)で補う。
- snapshot は `{"label":…,"individual":{…},"calc":{…}}`。**`calc` が無い(省略・null)ときは `"calc"` キーを出さない**ので、
  バイト列は ADR-0227 の正規形と同一で、既存行の `snapshot_hash` と重複判定は変わらない(テストでバイト列を固定)。
  `calc` の中のキー順は format, attacker, defender, moveId, field{weather, terrain, attackerScreens, defenderScreens},
  options{critical}(契約のプロパティ順)。
- 応答の `Favorite.calc` は正規化後の形(`format`・`field` の全キー・`options` を必ず持つ)。`calc` の無い行は
  **キーごと省く**(null を返さない。Swift・TS の生成型で「無い」を `nil` / `undefined` として読ませる)。
- 冪等(ADR-0227 §2)の「同じ内容」は `label`・正規化した `individual`・正規化した `calc` の組。`calc` の有無・中身が違えば別の行。
- 保存済みの `calc` が読めない(形が壊れている)行があれば、一覧は 500 `internal`(ADR-0227 §2 の「読めない行」と同じ)。

### 3. 検証とサイズ上限

- 検証は `individual` と同じ範囲(SP 各 0..32・合計 66・ランク ±6・level 50・`speciesKey` の形式・`natureId` 非空・
  列挙の未知は `invalid_enum`)を `calc.attacker` / `calc.defender` に同じ実装で適用する。`format` / `weather` / `terrain`
  の未知は `invalid_enum`。契約に無いキーは `calc` の中(`field`・壁・`options`・個体)も含めて 400 `unknown_field`。
  新しいエラーコードは足さない(ADR-0208 §2)。
- **マスタとは照合しない**(`moveId` が実在するか・メガシンカの持ち物の制約など)。理由は ADR-0227 §2 と同じ
  (record-svc は pokedex-svc に依存しない・レギュレーション変更で保存済みのお気に入りが開けなくなるのを避ける)。
  復元した計算が `unknown_move` 等で 400 になることはあり、クライアントは計算の失敗として表示する。
- **範囲検証を二重定義しない**: SP・ランク・レベルの上限は engine(`engine.MaxSPPerStat`・`engine.MaxSPTotal` 等。
  calc-svc が使う正)を参照し、record の `individual` と `calc` の検証は1つの関数を通す。record-svc が engine
  (純粋な Go のライブラリ)を import するのは「calc-svc への依存」ではない(HTTP も DB も持たない)。
  ただしランクの上限は engine に定数が無い(engine は ±6 をリテラルで持つ)ので、record に `maxRank` として置く。
- **サイズ上限: 正規化後の snapshot が 4096 バイトを超えたら 400 `invalid_input`**(`calc` の有無にかかわらず)。
  - 現実的に最大の入力(ラベル30文字・両側に任意項目すべて・64文字の ID)で約 1.7KB。4096 は2倍以上の余裕。
  - ADR-0227 では「形が契約で閉じているので本文は 1KB 未満」として個別の上限を持たなかったが、ID の文字列は
    長さが無制限で、`calc` で個体が3つに増えると「1MiB までの任意文字列」を3倍入れられる経路になる。項目ごとの
    `maxLength` を足すより、保存する物の大きさ1か所で限る方が漏れが無い。
  - `calc` の無い入力にも効くのは「異常に長い ID」(4096 バイト超)だけで、正当なクライアントの挙動は変わらない。
- 上限100件・同時実行の扱い(`FOR UPDATE`)・端末分離・全削除・失効・`last_seen_at`(ADR-0227 §3〜§6)は変えない。
  store・migration は変えない(snapshot は JSON 列で、store は中身を解釈しない)。

### 4. ログ

`calc` の中身(技・相手の種族・持ち物)と本文はログに出さない(ADR-0209 §3。`label`・`individual` と同じ)。

### 5. クライアントへの依頼(Web: web-cf・iOS: ios-6f)

詳細は docs/ai-shared/decisions/2026-10-04-api-favorite-calc-team-name.md。要点:

- ピン留め: 計算画面で、今の計算要求(`CalcRequest`。計算 API に送ったものと同じ値)を `calc` に、一覧に出す個体
  (通常 `calc.attacker`)を `individual` に入れて `createFavorite` を呼ぶ。
- 復元: 一覧の `Favorite.calc` があれば、その値で攻撃側・防御側・技・場・形式・急所を画面に戻し、**そのまま
  `calcDamage`(`POST /api/calc`)を呼んで結果を出す**。`calc` が無い旧いお気に入りは従来どおり `individual` を
  攻撃側(または防御側)に読み込むだけ。
- 計算が 400(`unknown_move` 等。レギュレーション変更・マスタ更新で起こりうる)なら、入力を画面に戻したうえで
  計算の失敗として表示する(お気に入りを消さない)。

## 受け入れ条件

ADR-0227 の AC-F1〜AC-F12 は変えない(すべて有効)。この ADR で新設するもの:

- **AC-FC1** `calc` 付きの作成は 201 で、応答・一覧の `calc` は既定値を補った形(`format`・`field` の全キー・`options`)。
  任意項目(double・天候・フィールド・壁・急所・個体の任意項目)は往復で落ちない。
- **AC-FC2** `calc` を送らない(省略・null)作成は、応答・一覧に `calc` キーを出さず、snapshot のバイト列は ADR-0227 の
  正規形と同一。保存済みの旧い行もそのまま読め、`calc` を持たない。
- **AC-FC3** `calc` 付きの snapshot は決まった正規形(キー順・既定値の補完)で、既定値を明示した `calc` と省いた `calc` は
  同じお気に入り(2回目は 200・同じ id)。`calc` の有無・技・急所が違えば別のお気に入り。
- **AC-FC4** `calc` の中の範囲外は 400 `invalid_input`(境界ちょうどは通る)、`attacker` / `defender` / `moveId` の欠落・
  空の `moveId` も 400 `invalid_input`、`format` の欠落・列挙の未知は 400 `invalid_enum`、契約に無いキーは 400 `unknown_field`。
  拒否した要求は行を作らない。
- **AC-FC5** `calc` の ID はマスタと照合しない(存在しない技・種族・持ち物でも 201)。
- **AC-FC6** 正規化後の snapshot が 4096 バイトを超える作成は 400 `invalid_input`(`calc` の有無にかかわらず)。
  現実的に最大の入力は通る。実 TiDB で 4096 バイトちょうどの snapshot が保存・読み戻し・重複判定できる。
- **AC-FC7** 保存済みの `calc` が読めない行があれば一覧は 500 `internal`。
- **AC-FC8** `calc` 付きの作成(201・200)・一覧の要求と応答が契約どおり。契約の `FavoriteInput.calc`・`Favorite.calc` は
  `CalcRequest` への `$ref` だけで省略可、`individual` は両方で必須。Web・iOS の生成型で `calc` が `CalcRequest` そのもの
  (Web はさらに「計算 API の本文の型と同じ」)。
- **AC-FC9** `calc` の中身と本文はログに出さない。

## 却下した案

- **`individual` を任意にして「どちらか必須」にする**: §1 の表。互換と単純さで劣る。
- **お気に入りを別の資源(「計算のお気に入り」)として新設する**: 一覧・上限・削除・失効・全削除の件数(`RecordDeletionResult`)を
  二重に持つことになる。画面にとっても「お気に入り」は1つの一覧で、`calc` の有無で開き方が変わるだけ。
- **計算結果(ダメージ)も保存する**: マスタ・engine の修正で値が変わる。開いたときに計算し直す方が正しく、保存量も小さい。
- **`calc` を opaque JSON にする**: ADR-0227 §2 と同じ理由(中身・大きさを限れず、生成型の恩恵が無くなる)。
- **`individual` と `calc.attacker` の一致を強制する**: §1。得る物が無く、防御側を代表にしたい画面を妨げる。
- **項目ごとに `maxLength` を足す**: §3。既存の `Individual` の契約(計算 API と共有)まで変えることになり、漏れも出やすい。

## 未決事項

- `individual` を一覧の代表として何にするか(攻撃側・防御側)は各画面の判断に任せた。揃える要望が出たら、この ADR に追記する。
- 計算履歴(生のイベント)からお気に入りを作る経路は持たない(ADR-0227 §5 のまま。ピン留めは利用者の明示の操作だけ)。
