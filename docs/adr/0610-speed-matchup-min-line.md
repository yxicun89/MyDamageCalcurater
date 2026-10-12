# ADR-0610: 素早さ画面を「相手を選んで上を取る最低ライン」にする(表は画面から外す)

- 状態: 提案(2026-10-12。I-speed-4。ユーザー要望 ec 経由。§6 の未確定点はユーザー確認待ち。推奨案で受け入れ条件とテストを先に書いた)
- 日付: 2026-10-12
- 関連: ADR-0600 §3(素早さの式)、ADR-0601(表の 6 行)、ADR-0602(自分の位置)、ADR-0604(Web 画面)、ADR-0607(追い風・まひ・トリックルーム)、
  ADR-0608 / ADR-0609(Web の仮想スクロール・周辺パネル)、ADR-0517 / ADR-0527(iOS の遅延描画・周辺パネル)、ADR-0357(ランクのメーター)、
  ADR-0179(engine の FinalSpeed・CompareSpeed・CompareTurnOrder)、docs/plan/combo-calc.md(連続技。§7 Q2 で判定は廃止予定)

## 背景

ユーザーの要望(2026-10-12。ec 経由): 「素早さ表は要らないかもしれない。データとしては持っておいてよいが、画面は一覧表ではなく、
相手のポケモンと自分のポケモンを入力して『この場合ここまですれば上を取れる』を表示する形(ダメージ計算の画面のような使い方)。
自分のポケモンの素早さを入力した後の『これ以上』も表なしで分かるように。『準速○○族以上』のような表記も入れて分かりやすく」。

今の画面(ADR-0604・0608・0609、iOS は ADR-0517・0527)は「全員の表 + 自分の位置」で、相手 1 体に対して「どこまで振れば抜けるか」は
表を見て自分で数える必要がある。

「素早さを上げて倒せるか」(ダメージ・確定数・技の手順)は判定(judge)ではなく、計算画面の**連続技**(CB-6 Web / CB-8 iOS、
`POST /api/calc/combo`)が受け持つ(データレーン 01 の訂正。判定は CB-7 で廃止予定)。素早さ画面はダメージを出さない。

## 決定

### 1. 意味と表記(ドメインの定義)

前提は既存のまま: Lv50・個体値 31・SP 0〜32・性格補正 下降 / なし / 上昇。素早さの値は ADR-0607 の式
(実数値 → ランク → 追い風・スカーフを 4096 基準で連結して 1 回だけ五捨五超入 → まひ floor(×50/100))で、**engine.FinalSpeed と同じ値**
(§3)。特性は対象外(ADR-0607 のユーザー決定のまま)。

**用語**

- **族**: 素早さの種族値。「X 族」= 素早さ種族値 X のポケモン。
- 表の 6 行のプリセット(ADR-0601 §2)で「X 族」の素早さを呼ぶ:
  - **無振り X 族** = X + 20(SP 0・補正なし)
  - **準速 X 族** = X + 52(SP 32・補正なし)
  - **最速 X 族** = floor((X + 52) × 11/10)(SP 32・上昇補正)
  - **最速スカーフ X 族** / **最速+1 X 族** / **最速+2 X 族** = 最速 X 族にスカーフ / ランク +1 / +2
  - 相手側の追い風が入るときは、その値にさらに追い風を掛ける(ADR-0607 §2 の連結)。
- **上を取る**: 素早さだけで比べた行動順が先(`engine.CompareSpeed(自分, 相手, trickRoom) == self_first`)。
  - トリックルームなしは「自分 > 相手」、トリックルーム中は「自分 < 相手」(下を取る)。
  - **同速は上を取るに含めない**(行動順はランダム)。同速は別の行「同速」として必ず併記する(§6 Q1)。
  - 技の優先度は見ない(同じ優先度どうしの比較)。優先度込みの行動順は連続技の担当。

**出力 1: 上を取る最低ライン(相手を選んだとき)**

自分の種族値 B と自分の条件(ランク・スカーフ・自分側の追い風・まひ)を固定し、相手の想定(§2)ごとに相手の素早さ T を求める。
性格補正 n(下降 / なし / 上昇)ごとに、SP 0〜32 のうち

- `movesFirst(n)` = { SP | 上を取る }、`tie(n)` = { SP | 自分 == T }

を返す。素早さは SP について広義単調増加なので、どちらも**区間**になる:

- トリックルームなし: `movesFirst = [k, 32]`。**k が最低ライン**(「補正なしなら S k 以上で抜ける」)。
- トリックルーム中: `movesFirst = [0, k]`。**k が上限**(「補正なしなら S k まで振っても下を取れる」。最も遅いのは下降・S0 = 最遅)。
- 空なら「どう振っても上を取れない」。`tie` が 2 点以上になることがある(下降補正・まひ・マイナスランクで同じ値が続く)。

**出力 2: 「これ以上」(自分の素早さ v が決まったとき。表の代わり)**

表の 6 プリセット P ごとに、種族値 1〜255 を 3 つの区間に分ける(素早さの比較。ADR-0607 §4 と同じくトリックルームで入れ替えない):

- `slower(P)` = { X | P の X 族 < v } = [1, a]  →「**P a 族まで抜ける**」
- `tie(P)` = { X | P の X 族 == v }           →「**P b 族と同速**」(無いことが多い)
- `faster(P)` = { X | P の X 族 > v } = [c, 255] →「**P c 族以上に抜かれる**」

あわせて、使用可能なポケモン(read model)で各区間の数と、境界の例(遅い側で最も速い 1 体・速い側で最も遅い 1 体。同じ族なら pokemonId の昇順の先頭)。

**『準速○○族以上』の定義**: 自分の素早さ v に対して「**準速 c 族以上(のポケモン)には抜かれる**」の c = `faster(準速).min`
= min{ X | X + 52 > v }。対になる「準速 a 族まで抜ける」の a = `slower(準速).max`。同速の族があれば間に「準速 b 族と同速」。
「最速○○族以上」「無振り○○族以上」も同じ規則で、P を最速・無振りに替えたもの。トリックルーム中は意味が反転し、
「準速 c 族以上には先に動ける(下を取れる)」と読む(画面が 1 つの規則で言い換える)。

例(手計算。テストと同じ): 自分 v = 170 のとき 無振り 149 族まで抜ける・150 族と同速・151 族以上に抜かれる / 準速 117 族まで・118 族同速・119 族以上 /
最速 102 族まで・103 族同速・104 族以上 / 最速スカーフ 51 族まで・52 族以上(同速なし)/ 最速+2 25 族まで・26 族同速・27 族以上。

**表現の候補(出力 1)と推奨**

| 案 | 表示の例(自分 100 族 vs 相手 最速 130 族 = 200、スカーフ持ち) | 長所 | 短所 |
|---|---|---|---|
| **A(推奨)性格ごとの最小 SP** | 「上昇: S2 以上で抜ける / 補正なし: S14 以上 / 下降: S29 以上」(同速があれば「S○ で同速」) | 振る量がそのまま分かる。同速・トリックルーム・届かないを 1 つの規則で表せる | 想定 × 性格で数字が多い(→ 下の見出しで要約) |
| B プリセットのはしご | 「準速(スカーフ)で抜ける」 | 一言で分かる。表の語と同じ | SP の調整幅が分からない。「S14 で足りる」が消える |
| C 必要な値を『族』で | 「必要 201 = 準速スカーフ 82 族相当以上」 | 自分の種族に依らない | 自分を選んでいるのに間接的。「何を振ればよいか」に答えない |

推奨は **A**。理由: 要望の「ここまですれば上を取れる」は「どれだけ振るか」の答えで、A だけが SP の値で答える。B の一言は A から導ける
(補正なしの最小 SP が 0 なら「無振りで抜ける」、補正なし 32 で足りれば「準速で抜ける」、上昇で足りれば「最速なら S k で抜ける」、
空なら「最速でも届かない」)ので、各想定のカードの**見出し**に B の語を A から導いて出し、本文に A の数字を出す。
C の『族』表記は出力 2(「これ以上」)で使う(要望の『準速○○族以上』はここに当たる)。

### 2. API(speed の契約 0.7.0。追加のみで後方互換)

`/table`・`/position`・`/pokemon` は**変えずに残す**(データ・API は残す要望)。既存では足りない:
`/position` は表の行の数しか返さず、相手 1 体に対する最小 SP も、種族値の区切り(族)も返さない。クライアントで総当たりすると
式を Web・iOS の 2 か所に複製することになる(却下案)。次の 2 つを足す(`services/speed/api/openapi.yaml` 0.7.0 に記述済み):

- `POST /api/speed/v1/matchup` — body `{ self: {pokemonId, rank?, scarf?, tailwind?, paralysis?, current?: {sp, nature}},
  opponent: {pokemonId, assumptions?: [{preset} | {custom: {sp, nature, rank?, scarf?}}], tailwind?, paralysis?}, trickRoom? }`。
  応答は想定ごとに `opponentSpeed`・`natures`(下降・なし・上昇の 3 件。`movesFirst` / `tie` の SP 区間。無い区間は欄ごと省略)・
  `current`(`self.current` があるときだけ。`self_first` / `opponent_first` / `speed_tie` = engine.TurnOrder と同じ語彙)。
  想定の省略は **無振り・準速・最速・最速スカーフ** の 4 つ(相手の振りは普通分からないので並べて見せる。§6 Q2)。
- `POST /api/speed/v1/reach` — body は `PositionRequest` をそのまま(検証・判定順・`tableTailwind` も `/position` と同じ)。
  応答は `speed`・`pokemon?`・6 プリセットの `ReachLine`(`slower` / `tie` / `faster` の種族値区間・各区間の数・境界の例)。

判定順は既存と同じ(ヘッダー 400 → body 400 / 413 → read model なし 503 → 未知の pokemonId 422 → 200)。ヘッダー検証のミドルウェアも同じ。

### 3. 計算の置き場所: speed のコア(`services/speed/internal/speed/matchup.go`)

- 純粋な Go(I/O なし)で、`MovesFirstLines`・`Matchup`・`Reach` を置く。素早さの値は `Speed`、行動順は **`engine.CompareSpeed`** を使う。
- **新しい式を作らない**: `Speed` は `engine.FinalSpeed`(ADR-0179 §2。連続技の行動順と同じ式)と全組み合わせで同じ値であることを
  `engine_parity_test.go` で守る(スカーフは engine では `SpeedMods: [{always, 6144}]` の持ち物)。implementer は `Speed` の中身を
  `engine.FinalSpeed` の呼び出しに置き換えてよい(推奨。値は変わらない)。
- engine に置かない理由: 入力が speed の概念(read model・表のプリセット・相手の想定)で、総当たりも SP 33 × 性格 3 と小さい。
  素早さ画面はポケモン一覧のために speed-svc が必須で、オフライン(WASM)で動かす必要がない。サービスは自分の read model だけを読む(絶対ルール 4)。
- 調整の「素早さを上回る」目標(engine `SPGoalOutspeed`。ADR-0331・0177)は配分全体の探索の一部で、性格ごとの区間・同速・トリックルームを
  返さないため流用しない(数値は同じ式で一致する)。

### 4. 画面(Web・iOS で同じ体験)

- **入力**(計算画面と同じ「攻撃側 / 防御側」の並びで、左 = 自分・右 = 相手):
  - 自分: ポケモンカード(ピッカー)・ランク(ADR-0357 のメーター)・スカーフ・追い風・まひ・任意で「今の振り」(SP の増減ボタン + 性格の区切りボタン)
  - 相手: ポケモンカード・想定のチップ(無振り / 準速 / 最速 / 最速スカーフ を既定で全部 on、+1 / +2 / カスタムを追加できる)・追い風・まひ
  - 場: トリックルーム
- **出力 1**(相手を選んだとき): 想定ごとのカード。見出し = 相手の素早さと B の要約語(「最速 200 → 最速なら S2 で抜ける」)、本文 = 性格ごとの
  最低ライン(A)と同速の SP。「今の振り」があれば行動順のバッジ(先に動く / 同速 / 後に動く。色だけに頼らず文字も)。
  トリックルーム中は「下を取る」の語と上限 SP に切り替える。下降補正の行はトリックルーム中だけ出す(§6 Q5)。
- **出力 2**(自分の振りが決まったとき。相手を選ばなくても出る): 「これ以上」のリスト。プリセットごとに 1 行
  「最速: 102 族まで抜く ・ 103 族と同速 ・ 104 族以上に抜かれる(例: ○○)」。数字は大きく、族の語はバッジで。
- **連続技へのリンク**: 相手を選んでいるとき「この相手で連続技を計算」。計算画面の連続技(CB-6 / CB-8)へ、攻撃側 = 自分のポケモン
  (今の振りがあれば素早さの SP・性格・ランク・スカーフも)、防御側 = 相手のポケモン を渡す。素早さ画面はダメージ・確定数を出さない。
- **画面から外すもの**: 表(Web の `TierViewport` = ADR-0608 の仮想スクロール、iOS の `SpeedTableSection` = ADR-0517)と
  周辺パネル(ADR-0609 / ADR-0527)。出力 2 の境界の例が周辺パネルの役目(自分の上下に誰がいるか)を引き継ぐ。
  これらの ADR は本 ADR で**置き換え**(画面の部品と、その部品専用の純粋関数・テストは画面と一緒に外す。外す理由は各 PR に書く。
  API の `/table`・`/position` とそのテストは残す)。
- 常時アニメーションなし。画像が無ければタイプ色のエンブレム(既存のまま)。

### 5. 段階

1. speed-svc: コア(matchup.go)と HTTP(matchup / reach)。このコミットのテストを通す(I-speed-4a)
2. Web: 新しい画面(I-speed-4b)。表・周辺パネルを外す
3. iOS: 同じ画面(I-speed-4c)
4. gitops: speed の digest 更新(既存の手順)

## 6. ユーザーに確認したい点(未確定。推奨で先に進める)

- **Q1 同速の扱い**: 「上を取る」に同速を含めない(推奨)。同速は「S○ で同速」と別に出す。
- **Q2 相手の想定**: 既定で 無振り・準速・最速・最速スカーフ の 4 つを並べる(推奨)か、1 つだけ選ぶか。
- **Q3 『準速○○族以上』の向き**: 「自分の素早さに対して、準速○○族以上には抜かれる」(出力 2。推奨)の意味でよいか。
  それとも「自分を準速○○族相当以上にすれば(相手を)抜ける」(案 C)の意味か。
- **Q4 届かないときの提案**: 「最速でも届かない」のとき「スカーフなら S○」「+1 なら S○」を添えるか(推奨: 後続。今回は「届かない」と相手との差だけ)。
- **Q5 下降補正の行**: トリックルームなしでは出さない(推奨)。
- **Q6 表**: 画面から完全に外す(推奨)か、「全体の表を見る」の折りたたみで残すか。
- **Q7 連続技へ渡す内容**: 今の振り(SP・性格・ランク・スカーフ)も渡す(推奨)か、ポケモンだけか。

## 受け入れ条件

1. `MovesFirstLines` は性格 3 通り(下降・なし・上昇の順)の `movesFirst` / `tie` の SP 区間を返し、Speed + engine.CompareSpeed の総当たりと一致する
   (トリックルーム・スカーフ・ランク・追い風・まひ・全員抜ける / 抜けない端。範囲外の入力は sentinel エラー)。
2. `Matchup` は自分・相手を read model から引き、想定(省略時 4 つ・preset / custom の片方だけ・重複は拒否)ごとに相手の素早さと 1 の区間を返す。
   `self.current` があれば今の素早さと行動順(engine.CompareSpeed)を付ける。相手側の追い風・まひは全想定に掛かる。
3. `Reach` は 6 プリセットの順に、種族値 1〜255 の slower / tie / faster の区間・read model での数・境界の例を返し、総当たりと一致する。
4. `Speed` は全組み合わせで `engine.FinalSpeed` と同じ値(素早さ画面と連続技の数値が一致する)。
5. 契約 0.7.0: `POST /matchup`・`POST /reach` がヘッダー・body・503・422 を既存と同じ順で扱い、応答は契約どおり(未知の欄なし)。既存 API は不変。
6. Web・iOS: 入力(自分・相手・場)→ 出力 1・出力 2・連続技へのリンク。表と周辺パネルは画面に出ない。両方で同じ語と同じ規則。
7. `make speed-test speed-lint speed-build`(Web は `make web-lint web-test`、iOS は `make ios-test`)が通る。

## 却下した案

- **クライアントで総当たり**: 式が Web・iOS に複製され、engine の式と割れる。
- **engine に置く**: §3。
- **`/position` の応答に足す**: 既存の完全一致テストの期待値を書き換えることになり(ADR-0607 の却下理由と同じ)、入力の形(相手が要る)も違う。
- **表を残して強調だけ変える**: 要望(表なしで分かる)に合わない。
- **判定(judge)へのリンク**: 判定は廃止予定(combo-calc §7 Q2)。連続技へリンクする。

## 追記(2026-10-12): iOS の画面(I-speed-4c)

Web(I-speed-4b)と同じ体験・同じ語・同じ規則を iOS で作る。仕様とテストは先に書いた(失敗する状態)。

### 1. 層の分け方(ADR-0500 §1 のまま。ロジックは PokeCalcKit、View は描くだけ)

| 層 | 置き場 | 内容 |
|---|---|---|
| ドメインの型 | `SpeedMatchupTypes.swift` | `SpeedMatchupRequest`・`SpeedMatchup`・`SpeedReach` など(生成型に依存しない) |
| 境界 | `SpeedService`(`SpeedDomainTypes.swift`) | `matchup(_:)`・`reach(_:)` を追加。`table`・`position`・`pokemon` は残す |
| 通信 | `APISpeedService` | 生成クライアント(`getSpeedMatchup`・`getSpeedReach`)で実装。本文の省略規則は `APISpeedMatchupTests` |
| モック | `MockSpeedService` | 簡易な式で matchup・reach。シナリオ `matchup-error`(`upstream_unavailable`)を追加 |
| 状態 | `SpeedMatchupViewModel` | 入力 → 要求 → 状態(idle / loading / loaded / failed)。matchup・reach・ポケモン一覧は互いに独立 |
| 表示の整形 | `SpeedMatchupDisplayBuilder`・`SpeedMatchupLabels` | 見出しの語・性格の行・「これ以上」の言い換え(純粋関数) |
| View | `SpeedScreenView` ほか | 状態を描くだけ |

`SpeedViewModel`(表の画面の状態)は `SpeedMatchupViewModel` に**置き換える**(新旧を同じ型名にすると、旧テストが
新実装のコンパイルを壊すため、先に別名で作った)。置き換え後に旧型を削除する。名前は `SpeedMatchupViewModel` のまま使う。

### 2. 入力と出力(Web と同じ並び)

- 自分: ポケモン(ピッカー)・ランク・スカーフ・追い風・まひ・「今の振りを入れる」(トグル。入れると かんたん設定〈無振り / 準速 / 最速〉と SP のステッパー・性格の区切りが出る)。
- 相手: ポケモン(ピッカー)・想定のチップ(6 つ。既定で 無振り・準速・最速・最速スカーフ が選択)・追い風・まひ。カスタム想定は ViewModel が持つ(`setCustomAssumption`)が、画面の入口は後続(Web の画面に合わせて決める)。
- 場: トリックルーム。
- **ランクの部品**: iOS には ADR-0357 のメーターに当たる部品がまだ無い(計算画面は − / 値 / + のボタン)。今回は既存の `SpeedStepper`(−/値/+)を使い、メーター化は別タスク(iOS 全体のランク入力をそろえるとき)。色だけに頼らず、値は符号付きの文字で出す。
- 出力 1(想定ごとのカード): 見出し(B の要約語)・相手の素早さ・性格ごとの行(下降はトリックルーム中だけ)・同速の SP・今の振りがあるときの行動順のバッジ(文字つき)。
- 出力 2(「これ以上」): 6 プリセットの行。「P a 族まで抜ける・b 族と同速・c 族以上に抜かれる」、各区間の数、境界の例。トリックルーム中は reach を取り直さず、言い換えだけ切り替える(`SpeedMatchupLabels.reachSlower` / `reachFaster` の `trickRoom`)。
- 連続技へのリンク: `SpeedMatchupViewModel.comboAvailable`(既定 false)が true のときだけ出す。連続技(CB-8)が計算画面に入るまで出ない。値は `comboHandoff`(攻撃側 = 自分、防御側 = 相手、今の振りがあれば素早さの SP・性格・ランク・スカーフ)。
- 画面は 1 本の縦スクロール。固定高にしない(長い文言は折り返す。AX5 でも横にはみ出さない)。常時アニメーションなし。色は補助(バッジ・行は文字でも区別する)。
- reach の要求は「今の振り」から作る custom の位置(`tableTailwind` = 相手の追い風)。契約に相手のまひの欄は無いので、reach は相手のまひを反映しない(matchup だけ反映する)。

### 3. 失敗

- `upstream_unavailable`(gateway が speed に届かない 503。契約外の code)は「素早さのサーバーに接続できません」(`SpeedLabels.errorMessage`。対応済み)。契約の 503 は `ErrorCode` の列挙に無い code を読めず decode エラーになりうるが、画面の文言は同じ。
- matchup・reach・ポケモン一覧は独立に失敗を出す。入力は壊さない。次の入力で復帰する。キャンセルは失敗にしない。計算・構築への波及なし(絶対ルール 5)。

### 4. モックの固定事実(UI テストの期待値の元)

既存の簡易な式のまま(`MockSpeedMatchupTests` が手計算で固定)。自分 A(種族値 100)・相手 D(130):
無振り D = 150 に対し 補正なし S31 以上・上昇 S18 以上で抜ける(上昇 S17・補正なし S30 は同速)→ 見出し「準速で抜ける」。
自分 167(SP 32・上昇)のとき「これ以上」は 無振り 146 族まで・147 族と同速・148 族以上 / 準速 114 族まで・115 族と同速・116 族以上。
`reach` の自分 170 は §1 の手計算の例(無振り 149/150/151・準速 117/118/119・最速 102/103/104・最速スカーフ 51/なし/52・+2 25/26/27)と一致する。

### 5. 画面から外すものと、整理するコード・テスト(implementer が画面の置き換えと一緒に行う)

ルール 6(テストを消して通さない)に従い、**外す理由は「表と周辺パネルを画面から外す決定(本 ADR §4。ADR-0517・0527 を置き換え)で、その部品専用だから」**。PR とコミットメッセージに書く。API の `/table`・`/position` のクライアントとそのテストは残す。

| 種類 | 外す | 残す・使い回す |
|---|---|---|
| Core のコード | `SpeedTableNavigation.swift`・`SpeedNeighborhood.swift`・`SpeedNeighborhoodLabels.swift`・`SpeedViewModel.swift`(`SpeedFailure`・`SpeedLoadState`・`SpeedInput` は `SpeedMatchupViewModel.swift` へ移す)・`SpeedLabels` の表・周辺パネル・「自分の位置」の文言 | `SpeedDomainTypes`(table・position の型を含む)・`APISpeedService` の pokemon・table・position・`MockSpeedService` の table・position・`SpeedLabels` のプリセット名・性格名・エラー文言 |
| Core のテスト | `SpeedTableNavigationTests`・`SpeedNeighborhoodTests`・`SpeedViewModelTableTests`・`SpeedViewModelInputTests`・`SpeedViewModelAsyncTests`・`SpeedViewModelPickerTests`・`SpeedLabelsTests` の表・位置の文言の検査 | `APISpeedServiceTests`・`MockSpeedServiceTests`・`SpeedContractSyncTests`・`SpeedLabelsTests` のプリセット名・性格名・エラー文言。ピッカーの絞り込みは `SpeedMatchupViewModelInputTests` が引き継ぐ |
| App | `SpeedTableSection.swift`・`SpeedNeighborhoodSection.swift`・`SpeedScreenView.swift` の自分のカード・結果(作り直す) | `SpeedScreenControls.swift`(`SpeedPill`・`SpeedFlowLayout`・`SpeedStepper`・`SpeedEmblem`)・`SpeedPokemonSheet.swift`(自分・相手で共用に一般化) |
| UI テスト | `SpeedNeighborhoodUITests`・`SpeedScreenUITests` の表・境界線・遅延描画・実数値入力・モードのテスト | `SpeedScreenUITests` のうち下の「書き換え案」で残すもの |

### 6. 既存 UI テストの書き換え案(表前提の箇所)

新構成の受け入れ条件は `SpeedMatchupUITests`(3 本)と `SpeedMatchupLargeTextUITests`(1 本)に書いた。既存の素早さ関連は次のとおり。

| 既存のテスト | 扱い | 理由・移し先 |
|---|---|---|
| `SpeedScreenUITests.testOpenSpeedScreenFromRootShowsTheTable` | 削除 | 表が無い。導線は `SpeedMatchupUITests` の起動処理が確かめる |
| `testOpensAtLaunchWithTheEnvironmentVariable` | **残す**(変更なし) | `POKECALC_OPEN_SPEED_SCREEN_AT_LAUNCH` は新画面でも使う |
| `testTheLastFilterCannotBeTurnedOff` | 書き換え | 表の絞り込み → 想定のチップ(`speedAssumption-*`)。最後の 1 つは外せない(`speedAssumptionMinimumNotice`)。ViewModel は `SpeedMatchupViewModelInputTests` が見るので、UI は 1 本に絞ってよい |
| `testTrickRoomReversesTheTableOrder` | 削除 | 表の並びは無い。トリックルームは `SpeedMatchupUITests` の 2 本目が見る |
| `testRawValueBelowEveryRowDrawsTheBoundaryAtTheBottom`・`testInvalidRawValueShowsAMessageAndNoResult`・`testJumpToSelfBringsTheSelfRowIntoView`・`testTableRendersOnlyNearbyTiersAndReadsOutTheTotal` | 削除 | 実数値入力・境界線・遅延描画は画面に無い(ADR-0517 を置き換え) |
| `testPresetMaxWithScarfShowsATieAndHighlightsTheSameSpeedTier`・`testCustomModeShowsItsControlsAndAResult` | 削除 | 入力モードが無い。今の振りは 2 本目が見る |
| `testPokemonPickerFiltersByName` | **残す**(`speedPokemonButton` から開く。変更なし) | ピッカーは自分・相手で共用。識別子は同じ |
| `testTableFailureDoesNotBlockThePosition`・`testPositionFailureKeepsTheTable` | 削除 | table・position は画面で呼ばない |
| `testPokemonFailureStillAllowsRawInput` | 書き換え | 一覧が失敗しても画面は開き、失敗の表示(`speedPokemonError`)が出て他の入力が残る |
| `testCalcScreenStillWorksWhenSpeedFailsEntirely` | **残す**(`speedTableError` の待ち合わせを `speedPokemonError` に変える) | 絶対ルール 5。3 本目も同じ確認を含む |
| `LargeTextLayoutUITests.testSpeedScreenNoHorizontalOverflowAtAX5`・`testSpeedScreenResultAndTableNoHorizontalOverflowAtAX5`・`testSpeedPokemonSheetNoHorizontalOverflowAtAX5` | 削除し、`SpeedMatchupLargeTextUITests` に置き換え | 表の識別子(`speedTable`・`speedFilter-*`・`speedTier-*`・`speedBoundary`)が無くなる。入力・カード・「これ以上」・シートを見る |

### 7. 受け入れ条件(iOS。上の 6 に対応)

1. `APISpeedService.matchup` / `reach` が契約 0.7.0 の経路・本文(省略規則)・応答(欄ごと無い区間は nil)・エラー(契約の code、通信失敗、decode、未定義ステータス、cancel)で動く。
2. `SpeedMatchupViewModel` が入力から matchup / reach の要求を作り、どの入力がどの要求を取り直すか(相手 → matchup のみ、自分の条件 → 両方、相手の追い風 → 両方、相手のまひ・想定・トリックルーム → matchup のみ)の規則で動く。idle / loading / loaded / failed が遷移し、古い応答・cancel は反映しない。
3. 想定のチップは既定 4 つ、最後の 1 つは外せない(カスタムを含めて数える)。
4. 見出し・性格の行・「これ以上」の言い換え(トリックルーム含む)が `SpeedMatchupLabels`・`SpeedMatchupDisplayBuilder` の規則どおり。届かない・同速のみ・同速なしを文字で出す。
5. 画面に表・周辺パネル・旧入力が出ない。連続技が未実装の間はリンクが出ない。失敗は他の入力・計算画面に波及しない。AX5 で横にはみ出さず、固定高でない。
6. `make ios-test` が通る(新しい UI テストは `SpeedMatchupUITests`・`SpeedMatchupLargeTextUITests`)。

### 8. 実装結果(I-speed-4c)と Web との文言の差

実装は上の §1〜§7 のとおり。実装で決めた点:

- **語の正**: 固定のテスト(`SpeedMatchupLabelsTests`・`SpeedLabelsTests`)に書いた iOS の語を正とする。Web(`web/src/i18n/speed.ts`。feat/speed-web-matchup)との差は次のとおりで、どれも意味は同じ(Web 側を直す必要はない)。
  - 族の表記: iOS は数字と「族」の間に空白を入れない(「102族まで抜ける」)。Web は「102 族まで抜ける」(日本語の画面の読みやすさ。Web は空白あり)。
  - 見出し: 「無振りで抜ける」「準速で抜ける」は同じ。Web の「最速なら S18 以上で抜ける」「S12 まで振っても先に動ける」は、iOS では「最速なら S18で抜ける」「補正なしなら S12まで下を取れる」(規則=補正なしの最小 SP から導く、は同じ。空白と語尾だけ違う)。
  - 端末の語: iOS の失敗文言は「アプリを開き直してください」、Web は「ページを開き直してください」。`upstream_unavailable`(gateway が speed に届かない 503)は iOS が「素早さのサーバーに接続できません」(通信失敗と同じ文言)、Web は原因が分かる別の文言。
  - 入力欄の名前: iOS は「今の振りを入れる」、Web は「今の振りも入れる」(トグルの語尾)。
  - 連続技へのリンク・「これ以上」の見出し・想定のチップ名(調整名)は同じ。
- **ランク**: 既存の `SpeedStepper`(−/値/+。識別子 `speedRankValue` など)。メーター化は別タスク。
- **相手の想定のチップ**: 6 つのプリセット。カスタムは ViewModel(`setCustomAssumption`)が持つが、画面の入口は Web の画面と合わせて後続で決める(今は出さない)。
- **想定が 1 つの注記**: `assumptionMinimumNoticeVisible` はカスタムなしでチップが 1 つだけ残ったとき(外せないとき)だけ true。
- **reach の要求**: 自分と今の振りだけで決まり、相手を選ばなくても出る。相手のまひは反映しない(契約に欄が無い)。
- **連続技リンク**: `SpeedScreenView(service:onOpenCombo:)` の `onOpenCombo` が nil の間は出さない(既定 nil。`SpeedFeature` はまだ渡さない)。
- **削除したもの**: §5 の表のとおり。専用のコード・テストは「表と周辺パネルを画面から外す決定(§4。ADR-0517・0527 を置き換え)で、その部品専用になったため」削除した。API の `table`・`position` のクライアントとそのテストは残した。`SpeedLabels` の表・位置・実数値の文言も削除(`SpeedLabels.mode` だけは GlossaryTests が語を見るため残す)。

