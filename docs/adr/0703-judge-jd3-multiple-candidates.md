# ADR-0703: 判定(素早さ×ダメージ連動)JD3 複数の相手候補を一度に判定

- 状態: 採用(2026-09-23)
- 日付: 2026-09-23
- 関連: ADR-0700(JD0 基盤。上流の呼び方・エラーの正規化・「上流の事情を漏らさない」立場)、
  ADR-0701(JD1 の request/response・素早さの求め方・検査順・並列化しない決定)、
  ADR-0702(JD2 の `speedField`。トリックルーム・追い風)、docs/judge-design.md §3 JD3、
  ADR-0012(サービス境界。各サービスが自分の契約を持つ)、
  ルートの `api/openapi.yaml` の `BulkCalcResult` / `BulkCalcRow`(各行を自己完結にする前例)、
  `services/balance/api/openapi.yaml` の `members` / `threats`(1〜6 件の前例)

## 背景

JD2 までで `POST /api/judge/v1/outspeed-and-ko` は「自分 1 体 対 相手 1 体」の判定を返せるようになった。
JD3 は judge-design.md §3 の JD3、**複数の相手候補を一度に判定**する。攻撃側(自分・技)は 1 つに固定し、
相手候補(`Individual` の配列)を受け取って候補ごとに JD1+JD2 の判定を配列で返す
(TB4 の仮想敵診断と同じ「1 つに対し複数」の形)。

これは既存の endpoint の**破壊的変更**になる。request の `defender`(単数)を `defenders`(配列)に、
response の 5 つの単数の欄を `matchups`(配列)に置き換える。したがって JD3 で一番決めるべきものは
「破壊的変更を今やってよいか」と、「候補のどれかで失敗したときに何を返すか」の 2 点になる。

## 決定

### 1. request: `defender`(単数)を `defenders`(1〜6 件の配列)に置き換える

| 欄 | 必須 | JD2 からの変更 |
|---|---|---|
| `format` | ✓ | 変更なし |
| `attacker` | ✓ | 変更なし(攻撃側は 1 つに固定) |
| `defenders` | ✓ | **新設**。`Individual` の配列。`minItems: 1` / `maxItems: 6` |
| `defender` | ― | **削除**(未知の欄として 400 になる) |
| `moveId` | ✓ | 変更なし(すべての候補に同じ技で判定する) |
| `field` | | 変更なし(calc-svc へ転送。すべての候補に同じものが使われる) |
| `speedField` | | 変更なし(judge が解釈。§5) |

- **上限 6 件**は `services/balance` の TB4 が `members[]` / `threats[]` を 1〜6 件にしている
  「手持ちの数」の慣習に倣う。上限を設けないと、1 リクエストの逐次の上流呼び出し数
  (natures 1 + attacker の種族 1 + 候補の種族 N + calc N = **2 + 2N**)が際限なく増え、
  レイテンシが読めなくなる。ADR-0208 が calc の候補・観測件数に上限を置いたのと同じ立場。
- **下限 1 件**は、0 件を許すと「空の判定」という意味のない成功を返すことになるため。
- 生成コード(oapi-codegen の echo5 サーバー)は `minItems` / `maxItems` を検証しない
  (ADR-0208 が実測で確認済み)ので、件数は `internal/httpapi` が自分で検査する。

### 2. response: 単数の 5 欄を `matchups: Matchup[]` に置き換える

`OutspeedAndKoResponse` は `matchups` 1 つだけを持つ。`Matchup` は候補 1 件ぶんの判定結果で、
`defenderIndex` / `outspeeds` / `speedTie` / `attackerSpeed` / `defenderSpeed` / `ko` を持つ。

- `matchups` は `defenders` と**同じ順序・同じ件数**で返す。i 番目の `defenderIndex` は必ず i になる。
- **`defenderIndex` を持たせる**のは、行を並べ替えて表示する画面(速い順・倒せる順)が
  request の候補との対応を取り違えないため。同じ `speciesKey` の候補が重複していても
  (§6)、index なら一意に指せる。
- **`attackerSpeed` を各行に持たせる**。攻撃側は 1 つに固定なのでどの行でも同じ値になるが、
  行だけを見て「何対何で抜けているか」が分かる形にする。ルートの `api/openapi.yaml` の
  `BulkCalcRow` が各行に防御側の実数値(`BulkDefender`)を持たせているのと同じ形で、
  ADR-0701 §1 が `attackerSpeed` を返すと決めた理由(画面が「なぜその判定なのか」を出せる)を
  行の単位でも保つ。トップレベルに 1 つだけ置くと、画面が行とルートの 2 か所を見る必要が出る。

### 3. エラーは最初に失敗した候補で全体を打ち切る(部分成功を返さない)

どれか 1 件の候補で失敗したら、その時点で request 全体をエラーにする。成功した候補ぶんの
`matchups` を返したり、行ごとにエラーを持たせたりはしない。

- judge は**薄いオーケストレーション層**(judge-design.md §2)である。部分結果の組み立ては
  「成功した行とエラーの行が混ざった応答」という新しい型と、それを読む画面側の分岐を生む。
  JD3 の目的は「1 回の入力で確認する」ことで、入力に不備があるなら直して出し直せばよい。
- 打ち切ることで、上流への無駄な呼び出しも減る(ADR-0700 §3「端末 ID が空の要求は上流を
  呼ぶ前に止める」と同じ立場)。
- **エラーの message には、どの候補で失敗したかを `defenders[<index>]` の形で入れる**。
  index が無いと、候補 6 件の request で「どれを直せばよいか」が分からない。
  これは ADR-0700 §3 の禁止(上流の URL・host:port・ホスト名・本文を文面に入れない)には
  当たらない。index は **judge が受け取った request 自身の情報**であって、上流の事情ではない。
  attacker 側の失敗は `attacker` と書き、候補の index を騙らない。
- エラーの HTTP ステータスと `code` の対応表は ADR-0701 §6 から変えない(新しい `ErrorCode` を作らない)。
  「どの候補か」は message で表し、`code` の列挙を候補の有無で分けない。

### 4. 検査順(ADR-0701 §5 の拡張)

上流は引き続き**逐次**で呼ぶ(ADR-0701 §5 の理由をそのまま引き継ぐ: エラーの勝ち負けを
検査順で確定させる・上流への同時接続を増やさない)。

```
ヘッダー (400)
  → body が 1 つの JSON か・上限 8 KiB (400 / 413)
  → defenders の件数が 1〜6 か (400)
  → attacker と各 defender の sp・ranks・format・必須文字列の範囲
     (400。attacker を先に見て、次に defenders を index 昇順。最初に範囲外だった候補の index で止める。
      ここまで上流を 1 回も呼ばない)
  → natures (503。1 リクエストにつき 1 回だけ)
  → natureId が一覧に無い (422 unknown_nature。attacker → defenders を index 昇順で最初の 1 件)
  → attacker の種族 (422 unknown_species / 503)
  → defenders の種族 (index 昇順。最初に失敗した候補で打ち切る)
  → calc (index 昇順。最初に失敗した候補で打ち切る。400 / 503)
  → 200
```

- **件数の検査を中身の範囲検査より先**に置く。7 件目があって、かつ 1 件目の `sp` も範囲外、
  という request でどちらが返るかを決めるため。
- **種族を全件引き終えてから calc に進む**(候補ごとに「種族 → calc」を回さない)。
  こうすると「index 2 の候補がマスタに無い」が「index 0 の calc が 400」に勝ち、
  返るエラーが入力だけから決まる。候補ごとに交互に呼ぶと、同じ request が上流の状態で
  違うエラーを返しうる。
- **natures は候補が増えても 1 リクエストにつき 1 回**(ADR-0701 §4)。性格は 25 件の固定リストで、
  attacker と全候補を同じ一覧から解決できる。候補ごとに引くと上流への往復が N 倍になる。

### 5. `speedField` / `field` は 1 リクエストに 1 つで、全候補に同じように適用する

- `speedField.attackerTailwind` / `trickRoom` は攻撃側・比較に効き、`defenderTailwind` は
  **すべての候補に同じように**適用される。候補ごとに追い風の有無を変える機能は JD3 に含めない。
- 根拠: JD3 の問いは「この 1 体で、この場の状況のとき、どの相手を抜けて倒せるか」である。
  候補ごとに場を変えると、それは「別の場の別の判定」を 1 回の request に詰め込んだものになり、
  比較する意味が薄れる。必要になったら `Individual` 側の欄として足せる(`Tailwind` は
  ADR-0702 §4 で既にコアの `Individual` の欄になっているので、契約側を足すだけで済む)。
- `field`(calc-svc へ転送)も同じく 1 つで、すべての候補の計算に同じものを送る。

### 6. 同じ `speciesKey` の重複除去はしない

候補に同じ種族が複数あっても取りまとめず、候補の数だけ素直に種族を引き、calc を呼ぶ。

- 上限が 6 件なので、節約できるのは最大 5 回の往復にすぎない。
- 重複除去には「同じ種族だが調整(`sp` / `ranks` / `itemId` / `natureId`)が違う候補」を
  取り違えない仕組みが要る。種族の取得だけを共有しても、calc は調整ごとに呼ぶ必要がある。
  得られるものに対して分岐が増えすぎる。
- `services/balance` の `AnalyzeResponse.members` も「重複した pokemonId をそのまま残す」と
  契約に書いており、同じ立場。

### 7. なぜ破壊的変更を今やってよいか

`POST /api/judge/v1/outspeed-and-ko` の request / response を非互換に変える。

- **judge-svc を呼ぶクライアントがまだ 1 つも無い**。judge-design.md §3 の JD5(Web/iOS の画面)は
  未着手で、plan.md の JD5 も `[ ]` のまま。gateway も judge を経由しない(ADR-0700 §6-3 で
  judge は自分の Ingress を持つと決めたので、gateway の生成物にも judge の型は無い)。
  ルートの `api/openapi.yaml`(Web/iOS の生成元)にも judge の endpoint は無い(ADR-0701 §7)。
  したがって壊れるものが無い。
- judge-design.md §3 は JD5 を最後に置く理由を「先に画面を作ると API 変更のたびに作り直しになる」と
  書いており、**今このタイミングで契約を変えることは、その段階分けが意図した通りの動き**である。
- 逆に、互換のために単数の `defender` を残すと、同じ問いに 2 つの入口ができる(§却下した案)。
  クライアントが付く前の今が、入口を 1 つに保てる最後の機会になる。
- 版を上げる(`/v2/`)方法も採らない。`v1` を使っている利用者がいないのに版だけ増えると、
  誰も使わない `v1` の実装を保守し続けることになる。

### 8. `internal/judge`(純粋なコア)は変えない

JD3 はオーケストレーション層(`internal/httpapi`)の変更で閉じる。

- `judge.Speed(Individual)` と `judge.CompareSpeed(attacker, defender, SpeedField)` は
  「1 対 1 の比較」の関数として既に正しく、候補ごとに attacker を使い回して N 回呼べばよい。
  `CompareSpeedAll([]Individual)` のような配列版をコアに足すと、コアが「候補の並び」という
  request の都合を知ることになる。並びと index は契約の概念なので `internal/httpapi` が持つ。
- したがって `internal/judge` は引き続き I/O を持たず、JD2 のテストの期待値も 1 つも変わらない
  (CLAUDE.md 絶対ルール 6 に触れない)。

## 受け入れ条件(JD3)

1. `POST /api/judge/v1/outspeed-and-ko` が `defenders`(1〜6 件)を受け取り、`matchups` を
   **`defenders` と同じ順序・同じ件数**で返す。i 番目の `defenderIndex` は i。
   各 `Matchup` は `outspeeds` / `speedTie` / `attackerSpeed` / `defenderSpeed` / `ko` を持ち、
   `attackerSpeed` はどの行でも同じ値、`ko` は calc-svc の値をその候補ぶんそのまま転記する。
   JD1・JD2 が決めた素早さの求め方(ランク → 追い風・スカーフの連結 → 1 回だけ五捨五超入)と
   トリックルームの反転は、候補ごとに JD2 と同じ結果になる。
2. 1 リクエストの上流呼び出しは **natures 1 回 + attacker の種族 1 回 + 候補の種族 N 回 + calc N 回**で、
   種族と calc は index 昇順の逐次。候補が増えても natures は 1 回のまま。
3. `defenders` が 0 件・7 件以上・配列でない・欄そのものが無いときは、上流を 1 回も呼ばずに
   400 `invalid_request`。件数の検査は候補の中身の範囲検査より先。候補の `sp` / `ranks` が範囲外の
   ときも上流を呼ばずに 400 で、message は**最初に範囲外だった候補**の `defenders[<index>]` を示す。
4. JD2 までの単数の `defender` を送った request は 400 `invalid_request`(未知の欄)になる。
5. 候補のどれかで失敗したら request 全体がエラーになり、**それ以降の候補の種族・calc を呼ばない**。
   エラー応答に `matchups` は現れない(部分的な成功を返さない)。
   `unknown_species` / `unknown_nature` / calc 由来の `invalid_request` の message は
   `defenders[<index>]` を含み、attacker 側の失敗のときは `attacker` を示して候補の index を騙らない。
   どの message にも上流の URL・host:port・ホスト名・本文は現れない(ADR-0700 §3)。
6. `field` と `speedField` は 1 リクエストに 1 つで、すべての候補の判定・calc 呼び出しに
   同じものが適用される。`speedField` は引き続き calc-svc に送らない(ADR-0702 受け入れ条件6)。
7. 同じ `speciesKey` の候補が重複していても取りまとめず、候補の数だけ種族と calc を呼ぶ。
   調整が違えば `matchups` の行も違う値になる。
8. `services/judge/api/openapi.yaml` が自分のスキーマだけで閉じており、`make judge-gen` の生成物が
   最新で、`make judge-test` / `judge-lint` / `judge-build` が通る。`internal/judge` は I/O を持たないまま。

## テストの期待値

- 上流は JD0〜JD2 と同じ `httptest.Server` の架空の応答。候補を増やすために架空の種族
  `9003-000` / `9004-000` を足し、スタブは speciesKey ごとに種族値・失敗・`ko` を差し替えられるようにする。
  実マスタ・実データは使わない(CLAUDE.md のドメイン規約)。
- 複数候補の期待値は JD1・JD2 の手計算をそのまま使う(判定の式は 1 つも変わらない)。
  attacker 無振り無補正・種族値 100 = **120** に対し、候補の種族値 90 / 100 / 130 = **110 / 120 / 150** を
  並べると、1 リクエストで「抜ける」「同速」「抜かれる」が同時に出る。
- 順序と対応の検証は、候補ごとに違う `ko` を返すスタブで行う(全候補が同じ `ko` だと
  行の取り違えが緑のまま通る。ADR-0701 の critic 指摘「フィクスチャが同じだと取り違えを検出できない」と同じ轍)。
- 重複除去をしていないことは、**同じ `speciesKey`・違う調整**の 2 候補で確かめる
  (取りまとめると 2 行目が 1 行目の値になって落ちる)。
- 打ち切りは「呼ばれた speciesKey の並び」「calc に送られた `defender.speciesKey` の並び」で確かめる。
- judge は engine の式を複製しないので `make test-golden` の対象は増えない(ADR-0700〜0702 と同じ)。

## 却下した案

- **候補を並列に呼ぶ(`errgroup`)**: ADR-0701 §5 の理由がそのまま当てはまり、候補が増えるぶん
  悪化する。どの候補のエラーが返るかが実行ごとに変わって契約に検査順を書けず、テストも不安定になる。
  1 リクエストが上流に最大 6 本を同時にぶつける形にもなる。遅くて困ると**測れてから**別 ADR で。
- **部分成功を返す(行ごとに結果かエラーを持たせる)**: §3 のとおり、judge が薄いオーケストレーション層で
  なくなる。「1 行だけ 503 の応答」の HTTP ステータスを何にするかも決められない(200 にすると
  失敗が黙って通る)。
- **同じ `speciesKey` を重複除去して種族を 1 回だけ引く**: §6 のとおり、最大 6 件で節約できるのは
  数回の往復にすぎず、調整違いの取り違えを防ぐ分岐だけが残る。
- **単数の `defender` も受け付け続ける(どちらか一方を必須にする)**: 同じ問いに 2 つの入口ができ、
  「両方指定されたら」「どちらも無かったら」の分岐と、response も単数・配列の 2 形態が要る。
  §7 のとおりクライアントがまだ無いので、互換のために複雑さを買う理由が無い。
- **`/api/judge/v2/outspeed-and-ko` を新設して v1 を残す**: §7 のとおり、利用者のいない v1 を
  保守し続けることになる。
- **`matchups` ではなく `defenderIndex` をキーにしたオブジェクトで返す**: JSON のオブジェクトは
  順序を保証しないため、「速い順に並べたい」画面が request の順序を復元できない。配列にする。
- **`attackerSpeed` をトップレベルに 1 つだけ置く**: §2 のとおり、行だけを見て判定の根拠が
  分からなくなる。`BulkCalcRow` の前例に倣って行を自己完結させる。
- **候補ごとに `speedField` を持たせる(候補ごとの追い風)**: §5 のとおり、比較の意味が薄れる。
  必要になってから `Individual` 側の欄として足せる。
- **上限を 6 件より大きくする(例: 12 件で裏の 6 体も)**: 逐次の上流呼び出しが 2+2N なので、
  12 件だと 1 リクエストで 26 回になりレイテンシが読めない。まず 6 件で使ってみてから決める。
- **`internal/judge` に配列版の `CompareSpeedAll` を足す**: §8 のとおり、コアが request の並びという
  契約の都合を知ることになる。
