# ADR-0704: 判定(素早さ×ダメージ連動)JD4 相手の技を含めた返り討ち判定

- 状態: 採用(2026-09-24)
- 日付: 2026-09-24
- 関連: ADR-0700(JD0 基盤。上流の呼び方・エラーの正規化・「上流の事情を漏らさない」立場)、
  ADR-0701(JD1 の request/response・素早さの求め方・検査順・並列化しない決定)、
  ADR-0702(JD2 の `speedField`。トリックルーム・追い風・補正の連結と丸め)、
  ADR-0703(JD3 の `defenders` / `matchups`・打ち切り・破壊的変更の根拠)、
  docs/judge-design.md §3 JD4、
  ADR-0105 §3 追記 / plan.md P3-7(API レーンが `GET /api/pokedex/moves/{key}`(`getMove`)を実装・main 統合。
  DECISIONS.md 2026-09-23)、ルートの `api/openapi.yaml` の `Move`(`priority: integer, default: 0`)、
  ADR-0602(同速を真偽値 1 つに丸めない前例)、ADR-0012(サービス境界)

## 背景

JD3 までで `POST /api/judge/v1/outspeed-and-ko` は「自分 1 体が、相手候補 1〜6 体を素早さで抜けて、
自分の技で倒せるか」を返せるようになった。JD4 は judge-design.md §3 の JD4、**相手の技を含めた返り討ち判定**を足す。
「抜けて倒せる」だけでは、**相手が先に動いて自分が落ちる**組み合わせを見落とすためである。

judge-design.md §3 の JD4 の定義はこうなっている: 「相手の moveId も受け取り、優先度(`priority`)と素早さから
先に動く側を決め、相手が先に動くなら相手→自分の順で `KOChance` を評価する」。

JD3 の時点でこの段階は**ブロックされていた**。技の優先度を judge が引く経路(技 1 件を ID で引く endpoint)が
pokedex-svc の公開 API に無かったためである(judge-design.md §3 の段階分けの根拠)。2026-09-23 に API レーンが
`GET /api/pokedex/moves/{key}`(`getMove`。`Move.priority` は `integer` / `default: 0`)を実装して main に統合したので
(DECISIONS.md・plan.md P3-7)、判定レーンは待たずに JD4 を出せるようになった。

したがって JD4 で決めるべきものは 4 つになる。(1) 候補ごとの技をどう受け取るか、(2) 先に動く側をどう決めるか、
(3) 逆方向(相手→自分)のダメージをどう計算するか、(4) 上流が 2 種類 × 2 方向に増えたときの検査順とエラー。

## 決定

### 1. request: `defenders` の要素を `Individual` から `DefenderCandidate` に変える(破壊的変更)

候補ごとに「その候補が撃ち返してくる技」が要るので、新しいスキーマ `DefenderCandidate` を作る。
中身は `Individual` の全欄(`speciesKey` / `natureId` / `sp` / `ranks` / `abilityId` / `itemId`)に
**`moveId`(必須)**を足したもの。

| 欄 | 必須 | JD3 からの変更 |
|---|---|---|
| `format` | ✓ | 変更なし |
| `attacker` | ✓ | 変更なし(`Individual` のまま) |
| `defenders` | ✓ | **要素の型が `Individual` → `DefenderCandidate`**(1〜6 件は変わらない) |
| `moveId` | ✓ | 変更なし(**攻撃側の技**。すべての候補に同じ技で判定する) |
| `field` | | 変更なし(calc-svc へ転送。§4 で逆方向だけ壁を入れ替える) |
| `speedField` | | 変更なし(judge が解釈。calc-svc には送らない) |

- **`Individual` 自体は変えない**。attacker は技を request 直下の `moveId` で 1 つだけ持ち、候補は自分の技を
  自分の欄で持つ、という非対称は意図どおり: 攻撃側は 1 つに固定(ADR-0703 §1)で技も 1 つ、候補は
  複数あって技も候補ごとに違う。`Individual` に省略可の `moveId` を足して両方に使い回すと、
  「attacker の `moveId` と request 直下の `moveId` のどちらが勝つか」という答えの無い分岐が生まれる。
- **`DefenderCandidate` は `allOf` で `Individual` を継承せず、欄を書き下す**。oapi-codegen の `allOf` の
  展開結果に契約の読みやすさを預けたくないのと、生成される型が素の struct のままになるため。
  欄の意味の正は引き続きルートの `api/openapi.yaml`(と `Individual` の description)である旨を書く。
- 候補の `moveId` が無い・空の request は、**上流を 1 回も呼ばずに** 400 `invalid_request`
  (検査順は ADR-0701 §5 の「必須文字列の範囲」の段。attacker → 候補を index 昇順)。

### 2. 先に動く側の決め方(`internal/judge` に新しい純粋関数を足す)

ゲームの一般ルール(第 3 世代から変わらない核心的な仕様)をそのまま実装する。

```
優先度が違う  → 優先度が高い方が必ず先に動く(素早さもトリックルームも見ない)
優先度が同じ  → 素早さで決まる(トリックルーム中は比較が反転する)
優先度も素早さも同じ → どちらが先か決まらない(tie)
```

- `internal/judge` に `CompareTurnOrder(attackerPriority, defenderPriority int, speed SpeedComparison) TurnOrder` を足す。
  `TurnOrder` は `AttackerMovesFirst bool` / `Tie bool` の 2 欄。
  - 優先度が違えば `AttackerMovesFirst = attackerPriority > defenderPriority`、`Tie = false`。`speed` は読まない。
  - 優先度が同じなら `AttackerMovesFirst = speed.Outspeeds`、`Tie = speed.SpeedTie`。
    `speed.Outspeeds` は ADR-0702 §3 で既に「自分が先に動くか」(トリックルームを反映済み)を意味しているので、
    judge はここでトリックルームを二度解釈しない。
  - **`Tie` が true のとき `AttackerMovesFirst` は false**。同速・同優先度はゲームでも行動順が乱数で決まり、
    どちらが先かを断定できない。真偽値 1 つに丸めない立場(ADR-0700 §6-1・ADR-0602)を、素早さだけでなく
    **優先度を含めた行動順**にもそのまま適用する。
- **`CompareSpeed` は意味を変えず残す**。あれは「自分が相手より(場の効果込みで)速いか」という素早さそのものの
  問いに答える関数で、ADR-0701 / ADR-0702 のテストの期待値は 1 つも変わらない(CLAUDE.md 絶対ルール 6)。
  先制判定は素早さとは別の問い(優先度を含む)なので、`CompareSpeed` に優先度の引数を足して意味を上書きせず、
  **結果を受け取る別の関数**にする。response も `outspeeds` / `speedTie` を残したまま
  `attackerMovesFirst` / `turnOrderTie` を足す(§3)。
- **これは `internal/judge`(純粋なコア)の拡張になる**。ADR-0703 §8 は「JD3 はコアを変えない」と決めたが、
  その理由は「配列の並びという契約の都合をコアに持ち込まない」ことだった。先制判定は契約の都合ではなく
  **ゲームのルールそのもの**で、judge が持つ数少ない計算の 1 つ(ADR-0701 §2 のスカーフと同じ立場)なので、
  コアに置く。`CompareTurnOrder` は引き続き I/O を持たず、引数と戻り値だけで閉じている。
- 優先度の値そのものは judge が決めない。`getMove` の `priority` をそのまま使う(技の表をコードに持たない。
  CLAUDE.md のドメイン規約)。

### 3. response: `Matchup` に反撃と行動順の欄を足し、`ko` を `attackerKo` に改名する

| 欄 | JD3 からの変更 | 意味 |
|---|---|---|
| `defenderIndex` / `outspeeds` / `speedTie` / `attackerSpeed` / `defenderSpeed` | 変更なし | JD1〜JD3 のまま |
| `ko` | **`attackerKo` に改名** | 自分の技が**この候補に**与えるダメージの確定数 |
| `defenderKo` | **新設** | この候補の技(`defenders[i].moveId`)が**自分に**与えるダメージの確定数 |
| `attackerMovePriority` | **新設** | 自分の技の優先度(`getMove` の `priority` をそのまま転記) |
| `defenderMovePriority` | **新設** | この候補の技の優先度(同上) |
| `attackerMovesFirst` | **新設** | 自分が先に動くか(§2。`turnOrderTie` が true のときは false) |
| `turnOrderTie` | **新設** | 優先度も素早さも同じで、どちらが先か決まらないか |

- **改名の理由**: 双方向の確定数が並ぶので、`ko` のままだと「誰が誰に」が読めない。`attackerKo` /
  `defenderKo` は「攻撃側の技の結果 / 防御側の技の結果」と 1 対 1 に読める。
- どちらの `KOChance` も **calc-svc の値をそのまま転記する**(judge は確定数を再計算しない。ADR-0701 §1)。
- **judge は「勝てる / 負ける」の真偽値に丸めない**。先制判定・双方の確定数という材料をそのまま返し、
  「相手が先に動いて自分が確定 1 発で落ちるから負け」という読みは画面が行う。丸めると乱数と確定の区別も、
  同速・同優先度の不確定さも消える(ADR-0700 §6-1 と同じ立場)。
- `attackerMovePriority` / `defenderMovePriority` を返すのは、`attackerMovesFirst` だけだと画面が
  「なぜ先に動くのか(速いからか、先制技だからか)」を説明できないため。ADR-0701 §1 が
  `attackerSpeed` / `defenderSpeed` を返すと決めた理由(なぜその判定なのかを画面が出せる)の、優先度版。

### 4. 逆方向の calc では `field` の壁(screens)を入れ替える

calc-svc の `field.attackerScreens` / `field.defenderScreens` は「**どちらの側に**壁が張られているか」を表す。
judge の request の `field` は、常に「自分の側 = `attackerScreens`・相手の側 = `defenderScreens`」として書かれている。

- 順方向(自分 → 相手)の calc は `field` をそのまま送る(ADR-0701 受け入れ条件 6。変更なし)。
- **逆方向(相手 → 自分)の calc では、`attackerScreens` と `defenderScreens` を入れ替えて送る**。
  逆方向では「攻撃側 = 相手・防御側 = 自分」になるので、自分の側の壁は calc-svc から見た `defenderScreens` になる。
- **天候(`weather`)と地形(`terrain`)は入れ替えない**。場全体の状態で、どちらが殴るかで変わらない。
- 入れ替えを忘れると、自分が張ったリフレクターが相手を守る側に適用され、**逆方向のダメージが静かに間違う**
  (応答はエラーにならないので、テストが無ければ気づけない)。JD4 で一番壊れやすい箇所なので、
  「順方向と逆方向で calc-svc に届いた `field` が違うこと」をテストで直接確かめる。
- `speedField` は順方向・逆方向のどちらにも送らない(ADR-0702 §1。変更なし)。

逆方向の calc の残りの欄も、単に**役割を入れ替える**だけにする: `attacker` にはその候補、`defender` には
自分(request の `attacker`)、`moveId` にはその候補の `moveId`、`format` は同じものを送る。

### 5. 上流の呼び出し順序(逐次。ADR-0701 §5・ADR-0703 §4 の拡張)

```
ヘッダー (400)
  → body が 1 つの JSON か・上限 8 KiB (400 / 413)
  → defenders の件数が 1〜6 か (400)
  → attacker と各 defender の sp / ranks / format / 必須文字列(moveId を含む)の範囲
     (400。attacker を先に、次に defenders を index 昇順。ここまで上流を 1 回も呼ばない)
  → 性格の一覧 (503。1 リクエストにつき 1 回だけ)
  → natureId が一覧に無い (422 unknown_nature。attacker → defenders を index 昇順で最初の 1 件)
  → attacker の種族 (422 unknown_species / 503)
  → attacker の技 (422 unknown_move / 503)
  → 【各 defender を index 昇順で: 種族 → 技】を全件(最初に失敗した候補で打ち切る)
  → 【各 defender を index 昇順で: 順方向 calc → 逆方向 calc】を全件(最初に失敗した候補で打ち切る)
  → 200
```

1 リクエストの上流呼び出しは **natures 1 + 種族 (1+N) + 技 (1+N) + calc 2N = 3 + 4N 回**(N は候補数。上限 6 で 27 回)。

- **同じ候補の中で「種族 → 技」を続けてよい**。ADR-0703 §4 が「候補ごとに種族 → calc を回さない」と決めた理由は、
  *異なる候補どうし*で違う種類の呼び出しが交互になると、返るエラーが上流の状態で変わることだった。
  同じ候補の中で 2 つを続ける分にはその問題は起きず、返るエラーは入力だけから決まる(index 昇順・最初の失敗で打ち切り)。
  一方、**候補 0 の calc が候補 2 の種族より先に走ってはいけない**ので、全候補の「種族 → 技」を終えてから calc に進む
  という 2 段構えは JD3 から変えない。
- **順方向 calc と逆方向 calc は同じ候補の中で連続させる**(候補 i の順方向 → 候補 i の逆方向 → 候補 i+1 の…)。
  同じ入力から作る 2 本で、間に他の候補を挟む理由が無い。順方向を先にするのは、失敗したときに
  「どちらの向きで失敗したか」が呼び出し順から一意に決まるため。
- **並列化しない**(ADR-0701 §5・ADR-0703「却下した案」)。呼び出しが 3+4N に増えたぶん並列化の誘惑は強くなるが、
  理由はそのまま当てはまる: どのエラーが勝つかが実行ごとに変わると契約に検査順を書けず、上流への同時接続も増える。
  遅くて困ると**測れてから**別 ADR で。

### 6. エラーの対応表に `unknown_move`(422)を足す

ADR-0701 §6 の表に 1 行足す。ほかの行は変えない。

| 起きたこと | HTTP | `code` |
|---|---|---|
| pokedex が `moveId` に 404(`ErrNotFound`) | **422** | **`unknown_move`** |
| pokedex が技の取得に 400・5xx・契約に合わない応答 | 503 | `upstream_unavailable` |

- `unknown_species` / `unknown_nature` と同じ「形は正しいが指しているものがマスタに無い」の区別(422)。
- message には `attacker`(攻撃側の技)または `defenders[<index>]`(候補の技)を含める(ADR-0703 §3 の形)。
  index は judge が受け取った request 自身の情報なので、ADR-0700 §3 の禁止(上流の URL・host:port・
  ホスト名・本文を文面に入れない)には当たらない。
- pokedex の 400 を `invalid_request` にしないのは ADR-0701 §6 の species と同じ理由
  (judge が事前検査を通した後に呼ぶので、400 は呼び出し側の非ではなく上流との契約ズレ)。

### 7. 攻撃側の技も上流で検証されるようになる(JD1〜JD3 からの変化点)

JD1〜JD3 では、攻撃側の未知の技は judge では検出できず、**calc-svc の 400 が `invalid_request` に畳まれて**
返っていた(ADR-0701 §6: judge は calc-svc の 400 の中身を見分けない)。JD4 は先制判定のために
attacker の技も `getMove` で引くので、**未知の技は calc に届く前に 422 `unknown_move` になる**。

- これは同じ入力に対する**応答の変化**(400 `invalid_request` → 422 `unknown_move`)である。
  クライアントがまだ無い(§8)ので今のうちに直し、テストにも明記する。
- 判定としてはこちらが正しい。「技の ID がマスタに無い」は入力の形の問題ではなく、
  指しているものが無い問題で、`unknown_species` / `unknown_nature` と同じ扱いになるのが筋が通る。
- calc-svc の 400 が `invalid_request` に畳まれる経路は**残る**(持ち物・特性・SP 超過など、
  judge が見分けられない理由はまだあるため)。

### 8. なぜ破壊的変更(`DefenderCandidate` と `ko` → `attackerKo`)を今やってよいか

ADR-0703 §7 の根拠がそのまま当てはまり、今回さらに短い。

- **judge-svc を呼ぶクライアントがまだ 1 つも無い**。JD5(Web/iOS の画面)は未着手で plan.md も `[ ]`。
  gateway は judge を経由せず(ADR-0700 §6-3)、ルートの `api/openapi.yaml` にも judge の endpoint は無い
  (ADR-0701 §7)ので、生成物にも judge の型は無い。壊れるものが無い。
- judge-design.md §3 が JD5 を最後に置いた理由は「先に画面を作ると API 変更のたびに作り直しになる」で、
  **今このタイミングで契約を変えることは、その段階分けが意図したとおりの動き**である。
- 逆に `ko` を残して `defenderKo` だけ足すと、非対称な名前(`ko` と `defenderKo`)が契約に残り続ける。
  クライアントが付く前の今が、名前を揃えられる最後の機会になる。
- `/v2/` を切る案も採らない(ADR-0703 §7 と同じ。利用者のいない v1 を保守し続けることになる)。

### 9. `internal/client` に技の取得を足す

`services/judge/internal/client/pokedex.go` に `Pokedex.Move(ctx, rc, key) (Move, error)` を足す
(`Species` / `Natures` と同じ実装の形: `send` → `decodeUpstreamJSON` → wire 型のポインタで欠落を検出 →
judge 独自の小さな DTO)。`GET {base}/api/pokedex/moves/{key}` を呼び、judge が読む欄
(`id` と `priority`)だけを取り出す。

- **`priority` が応答に無い場合は `ErrUpstreamInvalidResponse`**(0 に倒さない)。ルートの契約では
  `Move.priority` は `default: 0` で必須欄ではないが、judge にとってこれは **JD4 が依存する唯一の値**で、
  黙って 0 にすると先制技を普通の技として扱った判定を、正しい顔で返してしまう(ADR-0700 §4 の
  「欠けた欄を 0 で埋めない」・ADR-0701 §4 の「黙って無補正に倒さない」と同じ立場)。
  pokedex-svc の `GetMove` は常に `priority` を入れて返す(DECISIONS.md 2026-09-23 の実装)ので、
  実運用では起きない経路である。
- 威力・タイプ・分類は読まない(ダメージは calc-svc が計算する。judge は式を持たない)。
- `internal/client` は引き続き engine に依存しない(ADR-0701 §4)。

## 受け入れ条件(JD4)

1. **request**: `defenders` の各要素は `DefenderCandidate`(`Individual` の全欄 + **必須の `moveId`**)で、
   1〜6 件のまま。`attacker` は `Individual` のまま、攻撃側の技は request 直下の `moveId` のまま。
   候補の `moveId` が無い・空・文字列でない request は、**上流を 1 回も呼ばずに** 400 `invalid_request` で、
   message は最初に不正だった候補の `defenders[<index>]` を示す。
2. **response**: 各 `Matchup` が `attackerKo` / `defenderKo` / `attackerMovePriority` /
   `defenderMovePriority` / `attackerMovesFirst` / `turnOrderTie` を持ち、`ko` という欄は**無い**。
   `attackerKo` は順方向(自分 → その候補)、`defenderKo` は逆方向(その候補 → 自分)の calc-svc の
   `KOChance` をそのまま転記したもので、**取り違えていない**(候補ごと・方向ごとに違う値を返すスタブで確かめる)。
   `attackerMovePriority` / `defenderMovePriority` は `getMove` の `priority` の転記。
3. **先制判定**: `judge.CompareTurnOrder` が、
   (a) 優先度が違えば高い方を先にし、**素早さもトリックルームも見ない**、
   (b) 優先度が同じなら `SpeedComparison.Outspeeds`(トリックルーム反映済み)に従い、
   (c) 優先度も素早さも同じなら `Tie: true` / `AttackerMovesFirst: false` にする。
   `attackerMovesFirst` と `turnOrderTie` が同時に true になることは無い。
   既存の `outspeeds` / `speedTie` の意味と値は JD1〜JD3 から変わらない(素早さだけの比較であり続ける)。
4. **逆方向の calc**: 候補ごとに calc-svc を 2 回呼び、逆方向では `attacker` にその候補・`defender` に自分・
   `moveId` にその候補の技を送る。`field` を指定した request では、逆方向にだけ
   **`attackerScreens` と `defenderScreens` が入れ替わって**届き、`weather` / `terrain` は入れ替わらない。
   `field` を省略した request では、どちらの向きにも `field` を送らない。`speedField` はどちらにも送らない。
5. **検査順**: §5 のとおり。1 リクエストの上流呼び出しは natures 1 + 種族 (1+N) + 技 (1+N) + calc 2N。
   natures は候補が増えても 1 回のまま。全候補の「種族 → 技」を終えるまで calc を 1 回も呼ばない。
   どこかで失敗したら request 全体を打ち切り、それ以降の上流を呼ばず、部分的な `matchups` も返さない。
6. **`unknown_move`**: `moveId` が pokedex のマスタに無ければ 422 `unknown_move` で、message は
   `attacker` または `defenders[<index>]` を示す。**攻撃側の未知の技も JD4 からは 422 になる**
   (JD1〜JD3 の 400 `invalid_request` から変わる。§7)。技の取得が 400・5xx・契約に合わない応答なら
   503 `upstream_unavailable`。どの message にも上流の URL・host:port・ホスト名・本文が現れない。
7. **`internal/client`**: `Pokedex.Move` が `GET {base}/api/pokedex/moves/{key}` を呼び、
   `X-Device-Id` / `X-Session-Id` を呼び出し元のまま転送し、404 → `ErrNotFound` / 400 → `ErrInvalidRequest` /
   5xx・接続不能・タイムアウト → `ErrUpstreamUnavailable` / 契約に合わない 200(`priority` 欠落を含む)→
   `ErrUpstreamInvalidResponse` に正規化する。エラーの文面に上流の詳細を含めない。
8. `services/judge/api/openapi.yaml` が自分のスキーマだけで閉じており、`make judge-gen` の生成物が最新で、
   `make judge-test` / `judge-lint` / `judge-build` が通る。`internal/judge` は I/O を持たないまま。

## テストの期待値

- 上流は JD0〜JD3 と同じ `httptest.Server` の架空の応答。技も**架空の ID**(`test-move` /
  `test-defender-move-*` / `test-priority-move` 等)で、実マスタ・実データは使わない(CLAUDE.md のドメイン規約)。
- **方向ごと・候補ごとに違う値**を返すスタブにする。calc のスタブは `moveId` と `defender.speciesKey` の
  組で引けるようにし、順方向と逆方向で違う `ko` を返す。全部同じ値だと `attackerKo` と `defenderKo` の
  取り違えが緑のまま通る(ADR-0701 の critic 指摘・ADR-0703「テストの期待値」と同じ轍)。
- 先制判定の期待値は §2 の規則からの手計算。必ず置くケース:
  - **優先度が高い方が遅くても先に動く**(素早さでは負けている側が優先度 +1)。
  - **トリックルーム中でも優先度の勝敗は反転しない**(同じ組み合わせで `trickRoom: true` にしても
    `attackerMovesFirst` が変わらない)。一方、**同じ優先度なら**トリックルームで `attackerMovesFirst` が反転する。
  - **同速 + 同優先度で `turnOrderTie: true` / `attackerMovesFirst: false`**。
  - 負の優先度(例 -6)どうしの比較も高い方が勝つ(0 を特別扱いしない)。
- 壁の入れ替えは、`attackerScreens.reflect` と `defenderScreens.lightScreen` のように**左右で違う内容**を
  指定して確かめる(両側に同じ壁を張ると入れ替えても気づけない)。
- 素早さ・スカーフ・追い風・トリックルームの期待値は JD1〜JD3 から 1 つも変えない
  (CLAUDE.md 絶対ルール 6。JD4 は素早さの式を変えない)。
- judge は engine の式を複製しないので `make test-golden` の対象は増えない(ADR-0700〜0703 と同じ)。

## 却下した案

- **相手が先に動くときだけ逆方向の calc を呼ぶ(自分が先なら省略する)**: 呼び出しが減る代わりに、
  「自分が先に動いて倒しきれなかったとき、返しでどれだけ食らうか」という JD4 の主要な問いに答えられなくなる。
  同速・同優先度(tie)では「どちらが先か決まらない」ので、どちらにせよ両方の値が要る。
  さらに、返る `matchups` の欄の有無が上流の状態(速さ)で変わる契約になり、画面が分岐を持つことになる。
- **`field` を入れ替えずに逆方向を計算する**: §4 のとおり、自分の壁が相手を守る計算になり、
  エラーも出ないまま結果だけが静かに間違う。壁は場の各側に張られていて、どちらが殴るかで場所は変わらない。
- **先制判定を `internal/httpapi` に書く**: 行動順はゲームのルールで、契約や HTTP の都合ではない。
  `internal/httpapi` に置くと、優先度の規則が「上流を呼ぶ関数」の中に埋もれ、表駆動のテストで直接叩けない。
  ADR-0701 §2 がスカーフの丸めをコアに置いたのと同じ立場でコアに置く。
- **`CompareSpeed` に優先度の引数を足して `Outspeeds` の意味を「先に動くか(優先度込み)」に広げる**:
  ADR-0702 §3 が決めた `outspeeds` の意味を上書きし、JD1〜JD3 のテストの期待値が変わる
  (CLAUDE.md 絶対ルール 6 に触れる)。素早さそのものを知りたい画面(何対何で抜けているか)も答えを失う。
  問いが 2 つあるなら関数も欄も 2 つにする。
- **`attackerMovesFirst` を三値(`attacker` / `defender` / `tie`)の文字列 enum にする**:
  同速の扱いを真偽値 2 つ(`outspeeds` / `speedTie`)で表す既存の形(ADR-0700 §6-1)と食い違う。
  1 つの `Matchup` の中で 2 通りの表し方が混ざる方が読みにくい。
- **judge が「返り討ちに遭うか」の真偽値を返す**: §3 のとおり、乱数 n 発と確定 n 発の区別、
  同速・同優先度の不確定さが消える。judge は材料を渡し、判断は画面が行う。
- **`Individual` に省略可の `moveId` を足して attacker・defender の両方で使い回す**: §1 のとおり、
  request 直下の `moveId` との二重指定の分岐が生まれる。`DefenderCandidate` を別に作れば
  「候補は必ず技を持つ」を契約の `required` で表せる。
- **技の優先度を judge が持つ表から引く / 既定 0 で済ませる**: CLAUDE.md のドメイン規約(技をハードコードしない)に
  反する。`getMove` が main にある今、優先度をマスタから引かない理由が無い(JD3 の時点の「限定つきで進める」案は、
  ブロック解消により不要になった)。
- **技の取得を候補ごとに `errgroup` で並列化する / 順方向と逆方向の calc を並列に投げる**:
  ADR-0701 §5 の理由がそのまま当てはまる(エラーの勝ち負けが不定になる・上流への同時接続が増える)。
  1 リクエストが上流に最大 24 本を同時にぶつける形にもなる。
- **技の解決結果を 1 リクエストの中でメモ化する(同じ `moveId` の候補が複数いても 1 回だけ引く)**:
  ADR-0703 §6 が種族の重複除去を却下したのと同じ理由。上限 6 件で節約できるのは数回の往復にすぎず、
  「呼ばれた技の並び」で検査順を確かめるテストも読みにくくなる。必要だと測れてから足す。
- **`ko` を残して `defenderKo` だけ足す**: §8 のとおり、非対称な名前が契約に残り続ける。
