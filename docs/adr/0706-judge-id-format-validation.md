# ADR-0706: 判定の ID(moveId / natureId)の形式検証と URL への安全な埋め込み

- 状態: 採用(2026-09-25)
- 日付: 2026-09-25
- 関連: ADR-0700(JD0 基盤。上流の呼び方・エラーの正規化・「上流の事情を漏らさない」立場・
  「端末 ID が空の要求は上流を呼ぶ前に止める」)、
  ADR-0701(JD1 の検査順・エラーの HTTP ステータスと `code` の対応表)、
  ADR-0703(JD3 の帰属ラベル `attacker` / `defenders[<index>]`・最初の失敗で打ち切る)、
  ADR-0704(JD4 の候補ごとの `moveId`・`GET /api/pokedex/moves/{key}` の呼び出し・`unknown_move`)、
  ADR-0016 §2 / ADR-0017 §2(`services/balance` の `MoveId` / `AbilityId`。
  `^[a-z0-9]+(-[a-z0-9]+)*$` を契約と `internal/httpapi` の両方に置いた前例)、
  ADR-0012(サービス境界。pokedex-svc の key 検証は pokedex-svc が持つ)、
  issue #234(全体レビュー指摘。2026-09-24)

## 背景

`POST /api/judge/v1/outspeed-and-ko` は、request の `speciesKey` だけを形式で検査し
(`speciesKeyPattern`。ADR-0701 §5)、`moveId`(request 直下の攻撃側の技・候補ごとの技)と
`natureId` は **非空かどうかしか見ていない**。一方 `internal/client` は
`p.baseURL+"/api/pokedex/moves/"+key` のように key を素のまま URL に連結している。
この 2 つが重なると、クライアントの入力ミスが judge の外から見て**上流の障害に化ける**。

| 送られた `moveId` | 今の振る舞い | 何が問題か |
|---|---|---|
| タブ・`0x7f` などの制御文字 | `http.NewRequestWithContext` が `net/url: invalid control character in URL` を返し、`ErrInvalidRequest` に包まれるが、`writeMoveError` は `ErrNotFound` 以外をすべて `writeUpstreamError` に畳む → **503 `upstream_unavailable`** + `slog.Warn("judge upstream unavailable")` | クライアントの 400 を上流の障害として記録する。上流は健全なのに警告が出るので、**ログが運用の判断材料にならなくなる** |
| `x?y=1` / `x#y` | `/api/pokedex/moves/x` への問い合わせとして成立してしまう | **静かに別の技を引く**。判定は成功して返るので、外からは正しく見えたまま間違う |
| `a/b` | path 要素が 1 つ増え、別の route に当たる | 同上。404 になれば 422 `unknown_move` という誤った説明になる |
| 上限の無い長さ | body の上限(8 KiB。ADR-0701 §5)まで丸ごと path 要素になる | 上流へ出す URL の長さを judge が制御できない |

決めるべきものは 3 つある。**(a)** judge は ID の形式をどこまで検査するか、**(b)** その検査を
検査順のどこに置くか、**(c)** 検査を通った値をクライアントがどう URL に埋めるか。

## 決定

### 1. `moveId` / `natureId` は Showdown ID の形式(`^[a-z0-9]+(-[a-z0-9]+)*$`・1〜64 文字)で検査する

`internal/httpapi` が `moveIDPattern` / `natureIDPattern`(同じ正規表現)と長さの上限を持ち、
`speciesKeyPattern` と同じ場所・同じ形で検査する。

- 形式の根拠は **pokedex-svc のマスタの ID 命名**(Showdown ID: 小文字英数とハイフン区切り)で、
  `services/balance` が `MoveId` / `AbilityId` に同じ正規表現を置いた前例(ADR-0016 §2・ADR-0017 §2)を
  そのまま引き継ぐ。judge だけ別の綴りを許すと、同じマスタの ID にサービスごとに違う定義ができる。
- 正規表現は**長さを持たない**(`balance` の `moveIDPattern` と同じ)。長さは
  `len(id) > maxIDLength` で別に見る。正規表現に `{1,64}` を埋め込むと、契約の `maxLength` と
  正規表現の 2 か所に同じ数が現れて食い違いうる。
- 上限は **64 文字**。`balance` の 40 に揃えない理由は、judge が**マスタを持たないオーケストレーション層**
  だからである。balance は自分の read model を読んでいて「どの ID が存在するか」を知っているが、
  judge は知らない。judge の上限の役目は「マスタに無い ID を弾くこと」ではなく
  「**際限なく長い path 要素を上流へ出さないこと**」で、マスタに無い ID の答えは 400 ではなく
  422 `unknown_move`(ADR-0704 §6)である。上限を業務的に必要な長さぎりぎりまで詰めると、
  マスタ側が judge の知らない長い ID を持った日に「クライアントには直しようのない 400」になる。
  64 文字は現行のどの Showdown ID よりも十分に長く、かつ path 要素として読める範囲に収まる。
- **`abilityId` / `itemId` はこの ADR では検査しない**。どちらも URL に埋めず、calc-svc へ JSON の
  body で素通しするだけで(ADR-0701 §2・§3)、issue #234 の経路に乗らない。`itemId` は
  こだわりスカーフの ID との**完全一致**にしか使わないので、形式が違えば「一致しない」だけで
  静かな誤りにならない。契約の一貫性のために足すなら別 issue で、同じ `MoveId` / `NatureId` と
  同じ形にする。

### 2. 契約には名前付きスキーマ `MoveId` / `NatureId` を置く(`SpeciesKey` の前例)

`services/judge/api/openapi.yaml` に `MoveId` / `NatureId` を新設し、`moveId` / `natureId` が
現れる 4 か所(`Individual.natureId`・`DefenderCandidate.natureId`・`DefenderCandidate.moveId`・
`OutspeedAndKoRequest.moveId`)をその `$ref` に置き換える。

- 同じ契約の中で `SpeciesKey` が既にこの形(`pattern` を持つ名前付きの文字列スキーマを 4 か所から
  `$ref`)を取っており、生成物も `type SpeciesKey = string` という**別名**になるので、
  生成コードの型は実質変わらない(`MoveId = string` / `NatureId = string`)。
- `pattern` と `maxLength` を 4 か所に**書き写さない**。`DefenderCandidate` は `Individual` を
  `allOf` で継承せず欄を書き下しているので(ADR-0704 §1)、インラインで書くと同じ 2 行が
  4 か所に散り、片方だけ直す事故が起こる。
- 生成コード(oapi-codegen の echo5 サーバー)は `pattern` も `maxLength` も検証しない
  (ADR-0208・ADR-0703 §1 で実測済み)。**契約は説明で、実際に弾くのは `internal/httpapi`** という
  この契約の立て付けは変わらない。だから §1 の検査を実装側にも置く。

### 3. 形式の検査は上流を 1 回も呼ぶ前に終える(`writeUpstreamError` に到達させない)

形式が合わない ID は **400 `invalid_request`** で、`toOutspeedRequest` の中(natures を引くより前)で返す。
検査順(ADR-0701 §5・ADR-0703 §4・ADR-0704 §5)の中では、既存の「必須文字列の範囲」の検査に足す形になる。

```
ヘッダー (400)
  → body が 1 つの JSON か・上限 8 KiB (400 / 413)
  → defenders の件数が 1〜6 か (400)
  → attacker と各 defender の sp・ranks・format・必須文字列の範囲
     ← ここに moveId / natureId の形式・長さが入る (400。上流を 1 回も呼ばない)
  → natures (503) → 以下 ADR-0704 §5 のまま
```

- **長さの上限より body の上限(8 KiB)が先**。既存の `TestOutspeedAndKoRejectsOversizedBody` は
  9 KiB の `moveId` で **413 `request_too_large`** を期待しており、この順序が保たれる限り変わらない。
  64 文字の上限を body の検査より前に持ってくると、大きすぎる body が 400 に化けて契約が変わる。
- **`ErrUpstreamUnavailable` 系の経路に乗せない**。判定の分岐を `writeMoveError` の側で増やす
  (`ErrInvalidRequest` なら 400 にする)方法もあるが、それは「上流に投げてみて、URL が
  組み立てられなかったら 400」という後追いの構造で、上流を呼ぶ前に止める ADR-0700 §3 の立場
  (端末 ID が空の要求は上流に届く前に止める)と合わない。`writeUpstreamError` の警告ログは
  「上流が本当に不調なとき」だけに保つ。
- 形式が合っていて**マスタに無い**だけの ID は、今まで通り上流に問い合わせてから
  422 `unknown_move` / `unknown_nature`(ADR-0701 §6・ADR-0704 §6)。400 と 422 の線引きは
  「judge だけで判断できるか」で、ここは変えない。

### 4. エラーの message はどちら側の ID かを示す(ADR-0703 §3 の流儀)

attacker 側(request 直下の `moveId`・`attacker.natureId`)の失敗は `attacker`、候補側は
`defenders[<index>]` を message に含める。attacker 側の失敗で候補の index を騙らない。

- request 直下の `moveId` は**攻撃側が使う技**なので `attacker` と書く。現状この経路だけ
  帰属ラベルが付いていない(`toOutspeedRequest` が素の `errInvalidOutspeedBody` を返している)ので、
  合わせて付ける。
- 複数が不正なときは ADR-0703 §4 の順(attacker → 候補を index 昇順)で**最初の 1 件**を示す。
  request 直下の `moveId` は attacker の一部なので、候補の不正より先に返る。
- message に上流の URL・host:port・ホスト名・本文は入れない(ADR-0700 §3)。index と
  `attacker` は judge が受け取った request 自身の情報で、上流の事情ではない(ADR-0703 §3)。

### 5. `internal/client` は path 要素を `url.PathEscape` で埋める(二重の守り)

`Species` / `Move` が組み立てる URL の key を素の連結から `url.PathEscape(key)` に変える。

- §1 の検査を通れば `/` も `?` も制御文字も来ないので、これは**重複した守り**である。
  それでも入れるのは、この 2 つが**別の理由で別の場所にある**からである。§1 は「judge の契約に
  合わない要求を断る」検査で、§5 は「judge が組み立てる URL を、受け取った値の中身に関わらず
  構文として壊されないようにする」不変条件。§1 の正規表現が将来ゆるむ(マスタの命名が変わる・
  別の呼び出し元が付く)と、§5 が無ければ同じ穴がそのまま開く。
- `PathEscape` は「安全な文字はそのまま」なので、正常系の URL は 1 文字も変わらない
  (`test-move` → `test-move`)。既存のテストの期待値は変わらない。
- **`natureId` は URL に現れない**。`GET /api/pokedex/natures` は一覧をまとめて引いて judge の中で
  突き合わせる(ADR-0701 §4)ので、§5 の対象は `Species` / `Move` の key だけになる。
  それでも §1 で `natureId` を検査するのは、契約としての一貫性(同じ由来の ID が欄によって
  違う厳しさで通るのは説明できない)と、将来 `natures/{id}` のような endpoint が付いたときに
  同じ穴を開け直さないため。

### 6. pokedex-svc 側の key の検証は変えない(範囲外)

pokedex-svc が自分の `{key}` をどう検査するかは pokedex-svc の契約であり、データレーンの担当
(ADR-0012: 各サービスが自分の契約を持つ)。judge の変更範囲は
`services/judge/internal/httpapi`・`services/judge/internal/client`・`services/judge/api/openapi.yaml` に閉じる。

- judge が上流の検証に寄りかからない、という点は ADR-0700 §3 の立場(上流の事情を judge の
  外に漏らさない・上流の契約に引きずられない)と同じ。pokedex-svc が将来どう検査しても、
  judge の 400 は judge 自身の契約だけで決まる。

## 受け入れ条件(issue #234)

1. `moveId`(request 直下 = 攻撃側の技、および各候補の技)と `natureId`(attacker・各候補)が
   `^[a-z0-9]+(-[a-z0-9]+)*$` に合わない、または 64 文字を超えるとき、
   **上流を 1 回も呼ばずに 400 `invalid_request`** を返す。制御文字(タブ・`0x7f`)・空白・
   `/`・`?`・`#`・`%`・大文字・先頭や末尾のハイフンがいずれもこれに当たる。
2. その応答は 503 にならず、`slog.Warn("judge upstream unavailable")` も出ない
   (`writeUpstreamError` に到達しない)。
3. エラーの message は、attacker 側の失敗なら `attacker` を、候補側なら `defenders[<index>]` を含む。
   attacker 側の失敗で `defenders[` は現れない。複数が不正なら attacker → 候補 index 昇順で最初の 1 件。
   上流の URL・host:port・ホスト名・本文は現れない(ADR-0700 §3)。
4. 長さの境界: **64 文字ちょうど**の `moveId` / `natureId` は形式の検査を通り、
   マスタにあれば 200 になる。65 文字は 400。9 KiB の `moveId` は今まで通り **413 `request_too_large`**
   (body の上限が先。§3)。
5. 正常系は変わらない。既存の有効な ID(`test-move` のようなハイフン区切りの小文字英数)の
   応答・上流への呼び出し回数・URL は 1 つも変わらない。形式は合うがマスタに無い ID は
   今まで通り 422 `unknown_move` / `unknown_nature`。
6. `internal/client` の `Species` / `Move` は key を `url.PathEscape` で埋め、
   エスケープが要る文字を含む key でも **URL の構文として解釈させず**、literal な path 要素として送る
   (`net/url` のエラーで落ちない・クエリやフラグメントに化けない)。
7. `services/judge/api/openapi.yaml` に `MoveId` / `NatureId` があり、`moveId` / `natureId` の
   4 か所がそれを `$ref` している。`make judge-gen` の生成物が最新で、
   `make judge-test` / `judge-lint` / `judge-build` が通る。`internal/judge` は変えない
   (I/O を持たないまま・JD1〜JD4 のテストの期待値は 1 つも変わらない)。
   `web/src/judge/judge.gen.ts` は `make judge-gen` の対象外(judge レーンの生成物だが
   ルートの `make gen-ts` の持ち物。ADR-0705 §2 参照)なので、この契約を変えるときは
   `web/` で手動でも再生成する(`npx openapi-typescript ../services/judge/api/openapi.yaml
   -o src/judge/judge.gen.ts && npx prettier --write src/judge/judge.gen.ts`。
   ADR-0604 §2 の素早さレーンの前例に倣う)。忘れると型が契約と食い違ったまま残る。

## テストの期待値

- 異常系は `internal/httpapi` のテーブルテストで、**「不正な文字」×「4 つの欄」**の組み合わせで確かめる。
  欄は request 直下の `moveId`・`attacker.natureId`・候補の `moveId`・候補の `natureId` の 4 つ。
  片方の欄だけで確かめると、`toIndividualInput` は直ったが `toOutspeedRequest` 直下の `moveId` が
  残る(あるいはその逆)という取りこぼしが緑のまま通る。
- 候補側は**候補 2 件の request の index 1** を不正にして確かめる。index 0 だと、帰属ラベルを
  固定値 `defenders[0]` にしてしまう実装が通ってしまう(ADR-0703 のテストと同じ轍)。
- どの異常系でも `assertNoUpstreamCalls` を併せて確かめる。ステータスだけ見ると、
  上流を呼んでから 400 に畳む実装(§3 で却下した形)が通ってしまう。
- 長さの境界は **64 と 65 の両方**を置く。片側だけだと `>=` / `>` の取り違えが検出できない。
  64 文字ちょうどの `natureId` は、その ID を載せた架空の性格一覧のスタブで 200 まで通す
  (「一覧に無い」422 と混ざらないようにする)。
- `internal/client` 側は、エスケープが要る key を渡したときに `r.URL.EscapedPath()` が
  `"/api/pokedex/moves/" + url.PathEscape(key)` と一致することで確かめる。`r.URL.Path`
  (復号後)では `a%2Fb` と `a/b` が見分けられず、テストが通っても穴が残る。
- 上流はすべて `httptest.Server` の架空の応答で、架空の種族 `9001-000` / `9002-000`・架空の技
  `test-move` を使う(実マスタを使わない。CLAUDE.md のドメイン規約)。
- judge は計算式を持たないので `make test-golden` の対象は増えない(ADR-0700〜0705 と同じ)。

## 却下した案

- **`writeMoveError` / `writeSpeciesError` で `ErrInvalidRequest` を 400 に写す**: 一見すると
  1 行で済むが、§3 のとおり「上流に投げてみてから判断する」構造になる。`ErrInvalidRequest` は
  ADR-0700 §3 で「上流が 400 を返した」も表すので、**judge の入力が悪いのか上流が受け付けなかったのか**が
  混ざる。`?` や `#` を含む ID はそもそも上流呼び出しが成功してしまうので、この案では
  「静かに別の技を引く」方(issue #234 の 2 つ目の症状)が直らない。
- **`internal/client` の `PathEscape` だけで済ませる(httpapi の検査を足さない)**: 制御文字の 503 は
  消えるが、`x?y=1` は `x%3Fy=1` として上流に届き **404 → 422 `unknown_move`** になる。
  「その技はマスタに無い」という説明は嘘ではないが、本当の原因(ID の形式が違う)を隠す。
  クライアントの入力ミスは 400 で返す、という受け入れ条件そのものを満たさない。
- **`internal/httpapi` の検査だけで済ませる(`PathEscape` を入れない)**: §5 のとおり、
  検査と URL 組み立ては別の場所にある別の不変条件で、片方だけだと将来の変更で穴が開き直る。
  `PathEscape` は正常系の URL を 1 文字も変えないので、入れない理由が無い。
- **`balance` に合わせて上限を 40 文字にする**: §1 のとおり、judge はマスタを持たないので
  「40 で足りる」と言い切れる立場にない。足りなかったときの症状が「直しようのない 400」になる。
- **4 か所に `pattern` / `maxLength` をインラインで書く**(名前付きスキーマを作らない): §2 のとおり
  同じ 2 行が 4 か所に散る。`SpeciesKey` が既に名前付きになっており、揃える方が読みやすい。
- **`abilityId` / `itemId` にも同じ検査を足す**: §1 のとおり issue #234 の経路(URL への埋め込み)に
  乗らず、変更範囲も issue が「`moveId` / `natureId`」と定めている。契約の一貫性のためにやるなら
  別 issue で、`MoveId` / `NatureId` と同じ形に揃える。
- **judge 側では検査せず pokedex-svc の key 検証を強化する**: §6 のとおりサービス境界を越える
  (ADR-0012)。judge の `natureId` は URL に乗らないので pokedex-svc からは見えず、
  そもそも全部は塞げない。
- **正規表現に長さを埋め込む(`^[a-z0-9]{1,64}(-[a-z0-9]+)*$` 等)**: §1 のとおり契約の
  `maxLength` と二重管理になり、しかもこの書き方はハイフンを含む全体の長さを縛れない。
