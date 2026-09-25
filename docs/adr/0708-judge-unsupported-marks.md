# ADR-0708: calc-svc の「未対応」の印を判定の応答に、方向ごとに分けて中継する

- 状態: 採用(2026-09-25)
- 日付: 2026-09-25
- レーン: 判定(ADR 帯 `0700〜`)
- 関連: ADR-0123(engine / calc-svc の `unsupported: UnsupportedMark[]`。印の意味・値の一覧の正。
  §6-2・§7-2 で calc-svc の契約に入り、PR #372 で main 統合済み)、issue #271 / #270(印の由来)、
  ADR-0700(JD0 基盤。§4「欠けた欄を 0 で埋めない」・§3「上流の事情を漏らさない」)、
  ADR-0701(JD1。§1「確定数は calc-svc の値をそのまま転記する・judge は再計算しない」)、
  ADR-0704(JD4。§3 `attackerKo` / `defenderKo` の命名、§4 候補ごとに calc-svc を順方向・逆方向の
  2 回呼ぶ、§9「judge が依存する欄が欠けたら `ErrUpstreamInvalidResponse`」)、
  ADR-0706(§2 ルートの契約を `$ref` せず judge の契約に同じ定義を置く前例)、
  ADR-0012(サービス境界。各サービスが自分の契約を持つ)、docs/judge-design.md §2(薄いオーケストレーション層)

## 背景

ADR-0123 で engine と calc-svc は「この入力は engine が正しく計算できない」ことを
`unsupported: UnsupportedMark[]`(必須・印なしは `[]`)で返すようになった(多段技・威力変動・固定ダメージの技、
効果スキーマで表せない持ち物・特性、威力 0 の攻撃技)。数値そのものは印で変わらないので、
**印を読まない利用者には、誤った数値が正しい顔で届く**。

judge の `POST /api/judge/v1/outspeed-and-ko` はまさにその利用者である。`attackerKo` / `defenderKo` は
calc-svc の `KOChance` の転記(ADR-0701 §1)なので、例えば攻撃側の技が多段技なら
「確定 2 発」と書かれた値が実際には当てにならないのに、判定の応答からはそれが分からない。
JD5 の画面(ADR-0705)は受け取った値をそのまま出すので、**ユーザーには「確定 2 発で倒せる」と読める**。

judge は候補ごとに calc-svc を **2 回** 呼ぶ(ADR-0704 §4。順方向 = 自分の技 → その候補、
逆方向 = その候補の技 → 自分)。印はこの 2 本に**独立に**付く:

| 例 | 印が付く calc | 疑わしくなる欄 |
|---|---|---|
| 自分の技が多段技(`multi_hit`) | 順方向だけ | `attackerKo` |
| 候補の技が固定ダメージ(`fixed_damage`) | 逆方向だけ | `defenderKo` |
| 自分が防御側で効く持ち物(とつげきチョッキ等)を持っている | 逆方向だけ(自分が防御側になるのは逆方向。印は「その側で持つとダメージが変わる」側にだけ付く。ADR-0123 §4) | `defenderKo` |
| 双方の技が機構持ち | 両方 | 両方 |

したがって決めるべきものは 4 つある。**(a)** 印を応答のどの欄で返すか(方向をどう表すか)、
**(b)** 必須にするか省略可にするか、**(c)** judge の契約に `UnsupportedMark` をどう置くか、
**(d)** calc-svc の応答に `unsupported` が無かったときにどうするか。

## 決定

### 1. `Matchup` に `attackerKoUnsupported` / `defenderKoUnsupported` を足す(方向ごとに 1 つずつ)

| 欄 | 意味 |
|---|---|
| `attackerKoUnsupported` | **順方向**の calc(自分の技 → その候補。`attackerKo` を求めた計算)に付いた印 |
| `defenderKoUnsupported` | **逆方向**の calc(その候補の技 → 自分。`defenderKo` を求めた計算)に付いた印 |

- **名前は `<対応する確定数の欄> + Unsupported`** にする。この欄が説明するのは「どちら側のポケモンか」ではなく
  「**どの確定数が疑わしいか**」なので、対応する `attackerKo` / `defenderKo` の名前をそのまま前置し、
  1 対 1 で読めるようにする(ADR-0704 §3 が `ko` を `attackerKo` に改名したのと同じ「誰が誰に」を名前で表す立場)。
- **`attackerUnsupported` / `defenderUnsupported` にはしない**。`UnsupportedMark.target` には
  `defender_item` / `defender_ability` があり、**順方向の印にも防御側(= その候補)の持ち物・特性の印が入る**。
  「attacker の未対応」と読める名前は、中身(両側の持ち物・特性が混ざる)と食い違う。
  `attackerKoUnsupported` は「attackerKo という**見積もり**に付いた印」と読めて、中身と一致する。
- 接尾辞にするのは、生成物・契約・画面のどれでも `attackerKo` と `attackerKoUnsupported` が
  隣り合って並ぶため(`unsupportedAttackerKo` だと 2 つの欄が離れる)。

### 2. `KOChance` の中に入れない

`attackerKo` / `defenderKo` は `KOChance`(`$ref`)で、その意味は「**calc-svc の `KOChance` の転記**」
(ADR-0701 §1・judge の契約の `KOChance` の description)である。印は calc-svc の `KOChance` の中には無く、
`CalcResult` の直下にある(ADR-0123 §7-2)。

- `KOChance` に `unsupported` を足すと、judge の `KOChance` が「calc-svc の `KOChance` + judge が混ぜたもの」に
  なり、転記であるという性質が崩れる。同じ `KOChance` を将来別の用途で使うときにも引きずる。
- `Matchup` 直下の兄弟欄にすれば、`KOChance` は転記のまま保てる。

### 3. 必須の配列にする(印が無いときは `[]`。`null` も省略もしない)

`required` に 2 欄とも入れ、印が無いときは**空配列**を返す。

- calc-svc の契約が同じ形(必須・印なしは `[]`。ADR-0123 §7-2)なので、**中継する側が形を変えない**のが素直である。
  judge の応答だけ省略可にすると、画面は「欄が無い」「`null`」「`[]`」の 3 通りを同じ意味に畳む分岐を持つことになる。
- **`null` を返してはいけない**。Go の `nil` スライスは `null` に marshal されるので、
  「印が 1 つも無い」正常系がそのまま `null` になりうる。契約が `[]` と言っている以上、
  実装は空スライスを明示的に作る必要がある(テストで直接確かめる。「テストの期待値」参照)。
- 省略可(`omitempty` 相当)にして「無ければ印なし」と読ませる案は採らない。
  **欄が無いこと**が「印が無い」と「judge が古くて印を中継しない」のどちらなのか、画面から区別できなくなる。

### 4. 印は byte-for-byte で中継する(judge は解釈・並べ替え・間引きをしない)

`target` / `reason` / `id` を calc-svc が返したまま、**calc-svc が返した順のまま**返す。

- judge は薄いオーケストレーション層で、自分の計算式を持たない(docs/judge-design.md §2)。
  印の意味・条件・並びの正は ADR-0123(engine)にあり、judge がそれを再解釈すると正が 2 つになる。
- **judge は印を真偽値 1 つに丸めない**。「この見積もりは怪しい」だけにすると、画面が
  「何が未対応なのか(どの技・どの持ち物・どの理由か)」を説明できない。
  ADR-0700 §6-1・ADR-0704 §3 の「材料を渡し、判断は画面が行う」をそのまま適用する。
- **judge は `reason` の値を検査しない**。engine 側が新しい `reason` を足したとき(ADR-0123 §3 の
  「実装したら印の条件から外す」の逆向き)、judge が enum を知らないだけで 503 になるのは筋が通らない。
  judge の契約の enum は**説明**で、実際に弾かない(ADR-0706 §2 と同じ立て付け)。
- **順方向と逆方向の印を 1 つの配列にまとめない**。まとめると `target: move` の印が
  「自分の技」なのか「候補の技」なのか区別できなくなり、`attacker_item` が自分の持ち物なのか
  候補の持ち物なのかも読めなくなる(逆方向では役割が入れ替わる。§5)。
  `attackerKo` / `defenderKo` を別の欄で返しているのと同じ理由で、印も方向ごとに分けたままにする。

### 5. `target` の `attacker` / `defender` は **その calc から見た**役割である(契約に明記する)

逆方向の calc では「攻撃側 = その候補・防御側 = 自分」として送っている(ADR-0704 §4)。
したがって `defenderKoUnsupported` の中の `attacker_item` は**その候補の持ち物**、
`defender_item` は**自分の持ち物**を指す。

- judge が読み替えて `target` を書き換える案は採らない(§4 の byte-for-byte に反し、
  `UnsupportedMark` が calc-svc のものと同じ値集合でなくなる)。代わりに
  **契約の description で「どちらの個体を指すか」を欄ごとに書く**。ここを書き残さないと、
  画面が `defenderKoUnsupported` の `attacker_item` を自分の持ち物と読んで、逆の説明を出す。

### 6. judge の契約に `UnsupportedMark` を**複製**する(ルートを `$ref` しない)

`services/judge/api/openapi.yaml` に、ルートの `api/openapi.yaml` と**同じ** `UnsupportedMark`
(`target` / `reason` / `id`・同じ enum)を置く。

- judge は自分の契約を自分で持ち、ルートの契約を `$ref` しない(契約の冒頭の記述・ADR-0012)。
  この立て付けの前例は ADR-0706 §2(`MoveId` / `NatureId` を `services/balance` と同じ綴りで
  judge の契約に**別に**定義した)で、理由も同じ: サービスは独立にデプロイされ、契約も独立に版を持つ。
  ファイル跨ぎの `$ref` を使うと、judge の生成(`make judge-gen`)がルートの契約の版に縛られる。
- **複製の代償は「ルートが enum を足したとき judge が遅れる」こと**で、これは §4 のとおり
  judge が `reason` を検査しないので**応答が壊れる形では現れない**(知らない `reason` もそのまま通る)。
  遅れるのは契約の説明と生成物の型だけなので、ADR-0123 側で `reason` が増えたら
  この契約にも同じ値を足す(この ADR の「結果」に注意として残す)。
- 逆に、ルートの `CalcResult` が持つ `unsupported` の**意味の正はルートと ADR-0123**であり、
  judge の契約の description からそちらを参照する(`KOChance` が既に取っている形)。

### 7. calc-svc の応答に `unsupported` が無ければ `ErrUpstreamInvalidResponse`(→ 503)

`internal/client` に judge 自身の小さな DTO `UnsupportedMark{Target, Reason, ID string}` と
`CalcResult.Unsupported []UnsupportedMark` を足す(`KOChance` と同じ形: judge が読む欄だけを持つ)。
wire 側は `calcResultWire.Unsupported *[]unsupportedMarkWire` で、
**nil(欄が無い)なら `ErrUpstreamInvalidResponse`**、`[]` なら**空スライス**(nil にしない)にする。

- `[]` を nil に倒さないのは、nil のまま応答まで運ぶと `encoding/json` が `null` に marshal し、
  §3 で `[]` と決めた契約を実装が破るためである。「空である」ことを型で運び切る。

- ADR-0704 §9 が `Move.priority` に下した判断と同じ形である。「黙って印なしに倒す」と、
  **未対応の入力を『対応済み』と断言した応答**を、正しい顔で返してしまう。この ADR が防ごうとしている
  症状そのものを judge が再生産することになる。ADR-0700 §4「欠けた欄を 0 で埋めない」の適用でもある。
- 運用上の前提: calc-svc の契約では `CalcResult.unsupported` は**必須**で、実装も既に main にある
  (ADR-0123 追記・PR #372)。したがって「judge を先に出すと 503 になる」経路は**既に閉じている**。
  もし calc-svc を巻き戻すなら judge も巻き戻す(この依存は判定レーンの上流依存として plan.md に残す)。
- 各要素の `target` / `reason` / `id` も同じ扱いで、**どれかが欠けたら `ErrUpstreamInvalidResponse`**
  (`koChanceWire` が `hits` / `guaranteed` / `displayChancePercent` を全部要求しているのと同じ)。
  空文字列は「欠けている」と見なさない(engine が空の `id` を返すことは無いが、それは engine の不変条件で
  judge が検査する筋のものではない。judge が見るのは JSON の欄の有無)。

### 8. `internal/judge`(純粋なコア)は変えない

印は計算ではなく**素通しのデータ**なので、コアには置かない。
`internal/client`(上流の DTO)と `internal/httpapi`(応答への写し)だけで閉じる。
ADR-0703 §8 と同じ切り分け(契約の都合はコアに持ち込まない)。

## 受け入れ条件

1. **response**: 各 `Matchup` が `attackerKoUnsupported` / `defenderKoUnsupported` を持ち、どちらも
   **必須の配列**である。印が 1 つも無い正常系では **`[]`(空配列)**であり、`null` でも欄の欠落でもない。
2. **方向ごとに分かれている**: 順方向の calc が返した印は `attackerKoUnsupported` にだけ、逆方向の calc が
   返した印は `defenderKoUnsupported` にだけ入る。**取り違えない・混ぜない・相手の向きの印が漏れない**
   (方向ごとに違う印を返すスタブで確かめる)。片方だけに印があるとき、もう片方は `[]` になる。
3. **byte-for-byte の中継**: `target` / `reason` / `id` は calc-svc が返した値・**返した順**のまま。
   judge は並べ替え・重複除去・間引き・真偽値への丸めをしない。judge が知らない `reason` の値
   (契約の enum に無い文字列)も 503 にせずそのまま中継する(§4)。
4. **欠落は 503**: calc-svc の 200 の本文に `unsupported` が無い、または要素の `target` / `reason` / `id` の
   どれかが欠けているとき、`internal/client` は `ErrUpstreamInvalidResponse` を返し、
   endpoint は **503 `upstream_unavailable`**(既存の「`ko` が無い」と同じ経路)。
   message に上流の URL・host:port・ホスト名・本文は現れない(ADR-0700 §3)。
5. **候補ごとに独立**: 候補が複数あるとき、各行の印はその候補の 2 本の calc の印だけで決まる
   (他の候補の印が混ざらない)。
6. **既存の応答は変わらない**: `attackerKo` / `defenderKo` / 素早さ・優先度・行動順の欄の値、
   上流への呼び出し回数(3 + 4N)・順序・送る body は 1 つも変わらない
   (CLAUDE.md 絶対ルール 6。JD1〜JD5 のテストの期待値を変えない)。
7. `services/judge/api/openapi.yaml` に `UnsupportedMark`(`target` / `reason` / `id`。ルートと同じ enum)があり、
   `Matchup` の `properties` と `required` の両方に 2 欄が入っている。`make judge-gen` の生成物が最新で、
   `make judge-test` / `judge-lint` / `judge-build` が通る。`internal/judge` は変えない。
   **`web/src/judge/judge.gen.ts` を `web/` で手動再生成する**
   (`npx openapi-typescript ../services/judge/api/openapi.yaml -o src/judge/judge.gen.ts &&
   npx prettier --write src/judge/judge.gen.ts`。ADR-0706 受け入れ条件7 の再発防止。忘れると
   契約と型が食い違ったまま残る。critic が ADR-0706 の 1 回目で実際に検出した見落としである)。
   **`web/` で `npm run typecheck`(= ルートの `make lint` が含む)が通ること**: `Matchup` に必須欄を
   足すと、`web/src/judge/JudgeScreen.test.tsx`・`judgeClient.test.ts` が組み立てる既存の架空応答の
   リテラルが型として不完全になる(`npm run lint`〈eslint+prettier〉・`npm test`〈vitest。型検査しない〉
   では検出できない)。両ファイルの `Matchup` リテラルに `attackerKoUnsupported: []`・
   `defenderKoUnsupported: []` を足すこと(critic が本 ADR の round 1 で実際に検出した見落とし。
   ADR-0706 の再発防止を「再生成」だけに書いたのがこの見落としの原因だった)。
8. 画面(JD5・iOS)が印をどう見せるかはこの ADR の範囲外(ADR-0123 §6 と同じ切り分け)。
   judge は材料を返すだけで、`web/src/judge/` の画面の変更は別タスクにする。

## テストの期待値

- 上流は JD0〜JD5 と同じ `httptest.Server` の架空の応答。印の `id` も**架空**(`test-move` /
  `test-defender-move` / `test-item-vest` など)で、実マスタ・実データは使わない(CLAUDE.md のドメイン規約)。
- **共有スタブ(`calcBody`・`calcKO` の本文)に `"unsupported":[]` を足す**。§7 で欠落を 503 にするので、
  足さないと既存の全テストが 503 になる。既存テストの**期待値は 1 つも変えない**(印なしの正常系のまま)。
- **方向ごとに違う印を注入する**(`calcKO` と同じ `calcRoute` で引くスタブ)。順方向に
  `[{move, multi_hit, test-move}]`・逆方向に `[{move, fixed_damage, test-defender-move},
  {defender_item, unsupported_effect, test-item-vest}]` のように**件数も中身も違う**組にする。
  同じ印を両方向に返すと、取り違え・混合が緑のまま通る(ADR-0704「テストの期待値」と同じ轍)。
- **片側だけに印がある組**を必ず置く。もう片方が `[]` であることを確かめないと、
  「両方に同じ配列を入れる」実装が通ってしまう。
- **`null` と `[]` の区別は生の JSON で確かめる**。`api.Matchup` に decode した値では
  `nil` スライスと空スライスが `len() == 0` で見分けられない。応答の body を
  `map[string]any` に読み、欄が存在し、かつ `[]any{}`(`nil` ではない)であることを見る。
- 印の**順序**も確かめる(2 件以上の配列を並びごと比較する)。集合として比べると並べ替えを見逃す。
- `internal/client` 側は `TestDamageDecodesUpstreamResponse` と同じ形で、`unsupported` を含む本文から
  `CalcResult.Unsupported` が組み立てられること、`unsupported` の欠落・要素の欄の欠落が
  `ErrUpstreamInvalidResponse` になること(`TestDamageRejectsInvalidBody` の表に足す)。
- judge は計算式を持たないので `make test-golden` の対象は増えない(ADR-0700〜0707 と同じ)。

## 却下した案

- **印を 1 つの配列にまとめて `unsupported` として返す**: §4 のとおり、`target: move` が自分の技か候補の技か、
  `attacker_item` がどちらの個体の持ち物かが読めなくなる。「どの計算に印が付いたか分かる形」という
  この変更の目的そのものを満たさない。方向を表す欄(`direction: forward|reverse`)を各要素に足す案は、
  結局画面が配列を 2 つに振り分ける処理を持つことになり、`attackerKo` / `defenderKo` が
  既に方向で分かれているのと形が揃わない。
- **`KOChance` に `unsupported` を足す**(`attackerKo.unsupported` にする): §2 のとおり、
  judge の `KOChance` が calc-svc の `KOChance` の転記でなくなる。ネストが深くなるだけで、
  方向の区別という目的は兄弟欄でも同じく達成できる。
- **真偽値 1 つ(`attackerKoReliable: false` など)に丸める**: §4 のとおり、
  何が未対応なのか(技か持ち物か特性か・どの理由か)を画面が説明できなくなる。
  judge の既存の立場(材料を渡す。ADR-0700 §6-1・ADR-0704 §3)に反する。
- **judge が印を解釈して「表示すべき警告文」に変換する**: 文言はレーンごと・画面ごと(Web / iOS)に違い、
  judge がそれを決めると 2 つのクライアントが judge の文言に縛られる。
  日本語の文言は画面の持ち物(ADR-0705)。
- **省略可(`required` に入れない)にする**: §3 のとおり、欄の欠落が「印なし」と「judge が古い」の
  どちらなのか区別できない。calc-svc 側の形(必須 + `[]`)と食い違うのも説明しにくい。
- **calc-svc の `unsupported` が無いときは印なしとして扱う(503 にしない)**: §7 のとおり、
  この ADR が防ごうとしている「黙って正しい顔で返す」を judge 自身がやることになる。
  calc-svc の契約では必須で、実装も main にある以上、寛容にする利得が無い。
- **ルートの `api/openapi.yaml` の `UnsupportedMark` を `$ref` する**: §6 のとおり、
  judge の契約がルートの契約の版に縛られる(この契約の立て付けとファイル跨ぎ `$ref` の不採用は
  契約の冒頭・ADR-0012・ADR-0706 §2 で既に決めている)。
- **`reason` を judge 側で検証し、知らない値を 503 にする**: §4 のとおり、engine が enum を増やすたびに
  judge のデプロイが済むまで判定が落ちる。judge は印の意味を持たないので、検証する立場でもない。
- **`internal/judge` に「印があるか」を判定する関数を置く**: §8 のとおり素通しのデータで、
  計算が無い。コアに置くと引数と戻り値が同じ配列を往復するだけの関数になる。
