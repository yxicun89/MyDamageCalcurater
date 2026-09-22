# ADR-0701: 判定(素早さ×ダメージ連動)JD1 抜けるか+倒せるか

- 状態: 採用(2026-09-23)
- 日付: 2026-09-23
- 関連: ADR-0700(JD0 基盤。上流クライアント・エラー正規化・契約の分割方針)、docs/judge-design.md §1〜§4、
  ADR-0600 §3(こだわりスカーフ ×1.5 を「engine に無い小さな追加ロジック」としてサービスが持つ前例)、
  ADR-0602(同速を真偽に丸めない・検査順を契約に書く前例)、ADR-0010(`displayChancePercent` の意味)、
  ADR-0012(サービス境界。各サービスが自分の契約を持つ)、ADR-0105(pokedex の性格一覧)

## 背景

JD0 で judge-svc の骨格・上流クライアント(`Pokedex.Species` / `Calc.Damage`)・エラーの正規化まで揃った。
JD1 は judge の本体、「自分が相手より素早さで抜けるか + 使う技で相手を倒せるか」を 1 回の HTTP request で返す
`POST /api/judge/v1/outspeed-and-ko` を決める。ADR-0700 §5 の通り、endpoint は JD0 の契約に入れず JD1 で足す。

judge は計算式を持たない薄いオーケストレーション層である(judge-design.md §2)。JD1 で judge 自身が書くのは
「上流から集めた値を engine に渡して素早さを比べる」ところと、「上流の失敗を自分の言葉に直す」ところだけになる。

## 決定

### 1. request / response の形

**request**(`POST /api/judge/v1/outspeed-and-ko`、`X-Device-Id` / `X-Session-Id` 必須):

| 欄 | 必須 | 意味 |
|---|---|---|
| `format` | ✓ | `single` / `double`。calc-svc にそのまま渡す |
| `attacker` | ✓ | 自分の `Individual` |
| `defender` | ✓ | 相手の `Individual` |
| `moveId` | ✓ | 自分が使う技(1 つ)。JD1 は自分が攻撃する側だけを扱う(ADR-0700 §6-2) |
| `field` | | 天候・フィールド・壁。calc-svc にそのまま転送する。省略時は送らない |

`Individual` の欄は **judge-design.md §3 JD1 の列挙そのまま**(`speciesKey`・`natureId`・`sp`・`ranks`・`abilityId`・`itemId`)。
`ranks` は「技の追加効果を適用した後のランク」を呼び出し側が入れる(ADR-0700 §6-5)。

**response**(200):

| 欄 | 意味 |
|---|---|
| `outspeeds` | 自分の実数値が相手より**厳密に**大きいか(ADR-0700 §6-1) |
| `speedTie` | 実数値が等しいか(同速。`outspeeds` と同時に true にはならない) |
| `attackerSpeed` / `defenderSpeed` | judge が使った戦闘中の素早さ(ランク・こだわりスカーフ適用後) |
| `ko` | calc-svc の `KOChance`(`hits`・`guaranteed`・`displayChancePercent`)をそのまま転記 |

`attackerSpeed` / `defenderSpeed` を返すのは、ADR-0700 §6-1 が決めた 2 つの真偽値だけだと画面が「なぜその判定なのか」
(何対何で抜けているのか)を出せず、速度の数字を得るためにもう 1 本 API を呼ぶことになり、「1 回の入力で確認できる」という
目的(judge-design.md §1)が崩れるため。speed-svc の `PositionResponse` が戦闘中の素早さそのものを返すのと同じ立場。

`ko.chancePercent`(engine の生値)は返さない。ADR-0010 の通り**画面に出す値ではなく**、judge はこの値を読まない。

### 2. 素早さの求め方(judge が持つ唯一の計算)

順序は **実数値 → ランク → こだわりスカーフ**。

1. 実数値とランクは `engine.EffectiveStat(individual, engine.StatSpe)` をそのまま呼ぶ(`engine.RealStats` → ランク補正)。
   judge は式を複製しない(judge-design.md §2)。
2. こだわりスカーフだけは engine に無いので judge が持つ。`scarfSpeedModifier = 6144` を `engine.Modifier4096` 基準で掛け、
   五捨五超入 `(v*6144 + engine.Modifier4096/2 - 1) / engine.Modifier4096` で丸める。
   出典・式ともに ADR-0600 §3 / `services/speed/internal/speed/speed.go` と同一にする(2 つのサービスで答えが割れないため)。
3. `outspeeds = 自分 > 相手`、`speedTie = 自分 == 相手`。両方 false なら「抜けられている」。

**状態異常(麻痺など)による素早さの変化は JD1 では扱わない**(judge-design.md §3 の決定)。
したがって `Individual` に `status` を**置かない**。`status` を受け取れる形にすると「ダメージには効くのに素早さには効かない」
という説明のつかない半端な挙動になり、呼び出し側が誤った判定を正しいものと読んでしまう。`teraType` も同じ理由で JD2 に送る
(judge-design.md §3 JD1 の欄の列挙にどちらも無い)。

### 3. こだわりスカーフの持ち物 ID の決め方

judge は `Individual.itemId` の**文字列 1 つ**を既知の ID と比べてスカーフかどうかを決める。

- 既定値は `choicescarf`。pokedex-svc の持ち物 ID は Showdown の ID 規約(小文字英数のみ、記号を落とす。
  `services/pokedex/importer/util.go` の `toID`)で作られているため、"Choice Scarf" は `choicescarf` になる。
- ただし **決め打ちにしない**。`cmd/api` が環境変数 `JUDGE_CHOICE_SCARF_ITEM_ID` を読み、未設定・空なら
  `judge.DefaultChoiceScarfItemID`(`choicescarf`)を使う。実マスタの命名が違っていた場合に、コードを直さず
  overlay の環境変数 1 行で直せるようにするため(実マスタは Git に無く〈ADR-0002〉、この ADR の時点で実物を確認できない)。
- これは CLAUDE.md の「持ち物・技・ポケモンのリストをハードコードしない」に反しない。ここで持つのは**一覧ではなく、
  固定のゲームルール 1 つに対応する ID 1 つ**で、ADR-0600 §3 が `scarfSpeedModifier` をコアの名前付き定数 1 か所で
  持つと決めたのと同じ扱いになる。ダメージ側のスカーフの効果(技の固定)は calc-svc が持ち、judge は二重に適用しない
  (judge は `itemId` をそのまま calc-svc に転送し、judge 自身は素早さにしか使わない。calc-svc は素早さを返さない)。

### 4. 性格の解決(`Pokedex.Natures` の新設)

`engine.Individual.Nature` は `natureId` の文字列ではなく `Plus` / `Minus` の `StatKey` を要求する。judge は JD0 の時点で
性格を解決する手段を持っていないので、`internal/client` に `Pokedex.Natures(ctx, rc) ([]Nature, error)` を足し、
`GET {POKEDEX}/api/pokedex/natures`(ルートの `api/openapi.yaml` の `listNatures`)を呼ぶ。

- **1 リクエストにつき 1 回だけ呼び、attacker と defender の両方をその場で解決する**。
- これは ADR-0700 §2 が却下した「マスタ一式を取る」には当たらない。性格は全部で 25 件・増えない・レギュレーションに依存しない
  小さな固定リストで、種族のように「1 件ずつ引ける鍵」を持つ endpoint も無い(一覧しか無い)。2 体ぶんの解決に 2 回引くより
  1 回の一覧の方が上流への負荷も小さい。
- judge はマスタをキャッシュしない(ADR-0700 §2「リクエストごとに呼ぶ」)。起動時取得・TTL キャッシュは、鮮度と再取得の
  管理を judge が背負うことになるので、必要だと測れてから別 ADR で足す。
- `client.Nature` は engine に依存しない素の文字列(`plus` / `minus` は 6 ステータスキーか、無補正の `""`)にする。
  `internal/client` を engine から独立に保つため(現状も engine を import していない)。`engine.StatKey` への変換はコアが行う。
- 上流が `plus` / `minus` に 6 ステータス以外の値を返したら `ErrUpstreamInvalidResponse`。一覧が空でも同じ
  (黙って「無補正」に倒すと判定を静かに間違える。ADR-0700 §4 と同じ立場)。

### 5. 上流の呼び出し順序 — **並列化しない**

1 リクエストの中で上流を 4 回呼ぶ(natures 1 回・species 2 回・calc 1 回)。JD1 は**逐次**で呼ぶ。

固定の検査順(契約の description にも書く。ADR-0602 §4 の「check order」と同じ形):

```
ヘッダー (400) → body が 1 つの JSON か・上限 8 KiB (400 / 413)
  → sp・ranks・format・必須文字列の範囲 (400。ここまで上流を 1 回も呼ばない)
  → natures (503) → natureId が一覧に無い (422 unknown_nature)
  → attacker の species (404→422 unknown_species / 503) → defender の species (同じ)
  → calc (400 / 503) → 200
```

並列化しない理由:

- 上流は同じクラスタ内で、1 回あたりの往復は小さい。3 本の GET を並べても体感は変わらない一方、
  `errgroup` とキャンセルの取り回しが judge に増える。
- **どのエラーが勝つかが不定になる**。attacker と defender の両方の speciesKey が未知のとき、並列だと返る 422 の
  message が実行ごとに変わり、契約に検査順を書けず、テストも不安定になる。
- judge の 1 リクエストが上流に同時に 3 本ぶつける形になり、上流の並列数を judge の同時接続数 ×3 に押し上げる。
- 遅くて困ると**測れてから**、別 ADR で species 2 本だけ並列にする(natures → species×2 並列 → calc)。

`sp`・`ranks` の範囲検査を上流の前に置くのは ADR-0700 §3 の「端末 ID が空の要求は上流を呼ぶ前に止める」と同じ理由
(無駄な往復をしない・原因が分かりやすい)。

### 6. エラーの対応表(ADR-0700 §3 の続き)

| 起きたこと | HTTP | `code` |
|---|---|---|
| ヘッダー欠落、body が JSON でない・未知の欄、`sp` / `ranks` / `format` が範囲外 | 400 | `invalid_request` |
| body が 8 KiB 超 | 413 | `request_too_large` |
| `natureId` が natures の一覧に無い | 422 | `unknown_nature` |
| pokedex が speciesKey に 404(`ErrNotFound`) | 422 | `unknown_species` |
| 上流が未設定(client が nil)・接続不可・タイムアウト・5xx・契約に合わない応答 | 503 | `upstream_unavailable` |
| calc-svc が 400(`ErrInvalidRequest`) | 400 | `invalid_request` |
| 想定外の内部エラー | 500 | `internal_error`(固定文言) |

- `unknown_species` / `unknown_nature` を 422 にするのは、形は正しいが指しているものがマスタに無い、という
  balance / speed の `unknown_pokemon`(422)と同じ区別。
- calc-svc の 400 は `unknown_move` / `unknown_item` / `unknown_ability` / SP 超過などをまとめたもので、
  judge は**どれだったかを見分けられない**。ADR-0700 §3 で上流の本文を読まない・漏らさないと決めているため。
  したがって judge 側に `unknown_move` は作らず、`invalid_request` に畳んで「calc-svc が計算要求を受け付けなかった」
  という固定文言を返す。見分けが要ると分かったら、calc-svc の `ErrorCode` を読む(= 上流の契約に結合する)かどうかを
  別 ADR で決める。
- エラーの文面に上流の URL・host:port・ホスト名・本文を入れない(ADR-0700 §3。`assertNoUpstreamAuthority` で検査する)。
- 判定の endpoint が 503 を返しても `/healthz` は 200 のまま(ADR-0700 §5)。

### 7. 契約は judge が自分で持つ

`services/judge/api/openapi.yaml` に `Format`・`Individual`・`StatBlock`・`RankBlock`・`FieldState`・`Screens`・
`Weather`・`Terrain`・`SpeciesKey`・`KOChance` を judge 自身のスキーマとして定義する。ルートの `api/openapi.yaml` を
cross-file `$ref` しない(ADR-0012。balance・speed も同じ)。欄の**意味の正**はルートの `api/openapi.yaml` である旨を
description に書き、ルートは変更しない。

## 受け入れ条件(JD1)

1. `POST /api/judge/v1/outspeed-and-ko` が 200 で `outspeeds`・`speedTie`・`attackerSpeed`・`defenderSpeed`・`ko` を返し、
   `ko` は calc-svc の `hits` / `guaranteed` / `displayChancePercent` をそのまま転記している(judge は確定数を再計算しない)。
2. 素早さは 実数値(`engine.EffectiveStat`)→ こだわりスカーフ(×6144/4096・五捨五超入)の順で求まり、
   `itemId` が設定のスカーフ ID(既定 `choicescarf`)のときだけスカーフ補正が乗る。
   `outspeeds` は厳密な `>`、`speedTie` は `==`、遅いときは両方 false。状態異常は素早さに影響しない。
3. 1 リクエストにつき `GET /api/pokedex/natures` を **1 回だけ**呼び、attacker と defender の `natureId` を両方それで解決する。
   一覧に無い `natureId` は 422 `unknown_nature`、一覧が空・`plus`/`minus` が 6 ステータス以外は `ErrUpstreamInvalidResponse`(503)。
4. §5 の検査順で応答し、`sp` / `ranks` / `format` が範囲外の request では**上流を 1 回も呼ばずに** 400 を返す。
5. §6 の対応表どおりの HTTP ステータスと `code` を返し、上流が未設定(client が nil)なら 503 `upstream_unavailable`。
   どのエラーの文面にも上流の URL・host:port・ホスト名・本文が現れない。
6. `field` を指定した request では、calc-svc へ送る body の `field` がそのまま(weather・terrain・両側の壁)転送され、
   省略した request では `field` を送らない。
7. `services/judge/api/openapi.yaml` が自分のスキーマだけで閉じており(ルートの `api/openapi.yaml` への `$ref` が無い)、
   `make judge-gen` の生成物が最新で、`make judge-test` / `judge-lint` / `judge-build` が通る。

## テストの期待値

- 上流はすべて `httptest.Server` の架空の応答(架空の種族 `9001-000` / `9002-000`、架空の技・性格・持ち物 ID)。
  実マスタ・実データは使わない(CLAUDE.md のドメイン規約)。スカーフ ID だけは §3 の既定値そのものを使う。
- 素早さの期待値は engine の式から**手計算**する。JD1 で新しく増える計算はスカーフの丸めだけなので、
  丸めの向き(五捨五超入)が分かるケースを 1 つ必ず置く: 種族値 71・SP 0・無補正 → 実数値 91、×1.5 = 136.5 → **136**
  (四捨五入なら 137 になる)。確定数の期待値は calc-svc が担保済みなので judge では検証しない(転記されるかだけ見る)。
- 上流エラーの文面の検査には JD0 の `assertNoUpstreamAuthority`(`internal/client/client_test.go`)を再利用する。
- `t.Cleanup` の登録順は JD0 と同じ(`server.Close` を先に登録し、`done` の close を後に登録する = 後入れ先出しで
  close が先に走る)。
- judge は計算式を持たないので `make test-golden` の対象は増えない(ADR-0700 と同じ)。

## 却下した案

- **上流の 3 本の GET を並列に投げる**: §5 の通り、エラーの勝ち負けが不定になり契約に検査順を書けない。
  必要なら測ってから別 ADR で。
- **natures を起動時に 1 回取ってキャッシュする**: ADR-0700 §2 が却下した「マスタを抱える」に寄る。鮮度・再取得・
  起動ブロックの問題が戻ってくる。性格 1 件あたりの追加往復はレイテンシの支配項ではない。
- **性格を `natureId` の文字列パターン(`adamant` 等)から judge が直接引く**: 性格名とその補正の対応表を judge に
  ハードコードすることになり、CLAUDE.md のドメイン規約(マスタから引く)に反する。
- **`Individual` に `status` / `teraType` を持たせる**: §2 の通り、素早さでは無視しダメージでは効く半端な対応になる。
  麻痺の素早さ半減まで含めて JD2 で一緒に入れる。
- **`options.critical`(急所)を request に足す**: judge-design.md §3 JD1 の列挙に無い。`field` と違って
  「その技で倒せるか」の既定の問いを変えてしまう(急所前提の判定は別の問い)ので JD2 で扱う。
- **`ko` ではなく judge が「倒せる/倒せない」の真偽値に丸める**: 乱数 n 発を真偽に潰すと画面が確定と乱数を区別できない。
  ADR-0700 §6-1(同速を丸めない)と同じ立場で、`KOChance` をそのまま渡す。
- **calc-svc の `ErrorCode` を読んで `unknown_move` を見分ける**: ADR-0700 §3 の「上流の事情を漏らさない」を崩し、
  judge の契約が calc-svc の `ErrorCode` 列挙に結合する。
- **ルートの `api/openapi.yaml` に judge の endpoint を足す / cross-file `$ref` する**: ADR-0012・ADR-0700 §5 と同じ理由で却下。
