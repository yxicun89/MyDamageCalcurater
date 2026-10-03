# ADR-0415: iOS のタイプバランス画面(P6-21)— 段階分け・生成クライアントの追加・マスタの扱い

- 状態: 提案(spec-writer。受け入れ条件とテストが先。実装は implementer。2026-10-02)
- 関連: docs/type-balance-design.md §10・§11、ADR-0303(Web の画面)、ADR-0411(API 専用画面のマスタ・エラー文言)、
  ADR-0414(gateway 経由の `/api/balance/*`。PR #478)、ADR-0413(balance 0.8.0 のヘッダー検証)、
  ADR-0500(iOS の構成・生成)、ADR-0501(画面ごとの判断・LatestTaskRunner)、`services/balance/api/openapi.yaml`(契約の正)

## 背景
タイプバランスは Web のみ(設計 §11)。iOS にも出す。balance の契約は `api/openapi.yaml` とは別ファイルで、
schema 名(`Error`・`TypeId`・`Health` など)が衝突するため、既存の生成クライアント(`PokeCalcAPI`)にはそのまま混ぜられない。
別のセッションが `feat/ios-*` で RootView・project.pbxproj を触っているので、既存ファイルの変更は最小にする。

## 決定

### 1. 段階
| 段階 | 内容 | 今回 |
|---|---|---|
| 第1段 | チーム最大6体の防御相性表(`analyze`)+チーム集計+日本語の倍率表示(「×2 弱点」。設計 §10・Web と同じ文言) | する(P6-21) |
| 第2段 | 攻撃範囲(`coverage`。技を選んだメンバーの防御タイプ別の最大倍率・有効/抜群) | する(P6-21) |
| 第3段 | 仮想敵(`threats`)・おすすめタイプ(`recommendations`)・技範囲チェッカー(`move-range`) | する(P6-22。§8) |

### 2. 生成クライアントは別モジュール `PokeCalcBalanceAPI`
- `ios/tools/openapi-gen/openapi-generator-balance-config.yaml`(types + client、`accessModifier: public`、`namingStrategy: idiomatic`。
  既存設定と同じ。`filter` なし = balance の全操作)を足し、`services/balance/api/openapi.yaml` から
  `ios/PokeCalcKit/Sources/PokeCalcBalanceAPI/Generated/` に生成する(コミットする・手で編集しない)。
- `ios/scripts/openapi-gen.sh` は「名前|仕様|設定|出力先」の組の配列を順に処理する(`--check` も同じ)。`make ios-gen` / `make ios-gen-check` はそのまま。
  **再生成手順**: `services/balance/api/openapi.yaml` を変えたら `make ios-gen` → 生成物をコミット。ずれは `make ios-gen-check`(`make ios-test` に含まれる)が検出する。
- `Package.swift` に `PokeCalcBalanceAPI` ターゲット(`PokeCalcAPI` と同じ依存)を足し、`PokeCalcCore` とテストが依存する。View(`ios/PokeCalc`)は生成型に触れない。
- 生成元の版: この ADR の時点の main は balance 0.7.0。0.8.0(ADR-0413。`missing_header`/`invalid_header` を追加し `missing_request_context` を廃止)が
  main に入ったら `make ios-gen` で再生成する(列挙に無いコードを受けると生成クライアントがデコード失敗にするため。実装者は 0.8.0 の取り込み後に再生成してから仕上げる)。
- 却下: `PokeCalcAPI` に統合(schema 名の衝突。`api/openapi.yaml` を balance で汚す)/ 手書きの URLSession クライアント(契約の二重管理。CLAUDE.md 絶対ルール1)。

### 3. 呼び出し先は gateway
iOS は gateway の `/api/balance/*`(ADR-0414)を呼ぶ。契約のパスが `/api/balance/...` なので、`serverURL` は gateway の基点 URL(`POKECALC_API_BASE_URL`。計算・pokedex と同じ)で、
パスを足さない。ヘッダーは `ClientIdentity` の `X-Device-Id` / `X-Session-Id`(0.8.0 の正準 UUID 検証を満たす。`UUID().uuidString` は gateway の `isCanonicalUUID` が通す大文字形)。

### 4. マスタとフォールバック
- ポケモン・技・特性は既存の `PokeCalcService`(`searchSpecies`・`species(key:)`・`searchMoves`。構築編集と同じ取得口・同じ「先頭ページ ∩ learnset」規則)を再利用し、新しい取得口を作らない。
- **架空データへのフォールバックはしない**(Web の ADR-0411 と同じ)。モック構成(`POKECALC_USE_MOCK` / 接続先なし)では `UnavailableBalanceService` が
  `balance_unavailable` を返し、画面は「タイプバランスの API に接続できません」を出す(架空の相性表を作らない)。
- 倍率・集計・category・有効/抜群は balance の応答をそのまま表示し、iOS で計算し直さない(ADR-0303 §1)。

### 5. エラー文言
`BalanceErrorText.message(forCode:)` が Web の `balanceErrorText` と同じ日本語を返す。サーバーの英語の `message` は画面に出さない。
通信失敗・デコード失敗は `balance_unavailable`、未知のコードは汎用文言。Web との差は `missing_header`/`invalid_header`(0.7.0 互換で `missing_request_context` も同文言)の
「ページを開き直してください」を「アプリを開き直してください」にした1点だけ。

### 6. 構成(既存ファイルの変更を最小に)
- `PokeCalcCore` に新規ファイルだけを足す: `BalanceDomainTypes.swift`・`BalanceService.swift`(プロトコル + `UnavailableBalanceService`)・`APIBalanceService.swift`・
  `BalanceLabels.swift`(倍率・見出し・エラー文言・`BalanceScreenError`)・`BalanceViewModel.swift`。`PokeCalcService` プロトコルは**変えない**
  (全スタブ・モックへの影響と他セッションとの競合を避けるため、balance は別プロトコル `BalanceService`)。
- ViewModel は Foundation のみ(`@MainActor @Observable`)。stale 応答の抑止は呼び出しごとの世代で行い、連続操作の要求の連発は `LatestTaskRunner`(debounce)で抑える。
- View は `ios/PokeCalc/` に新規ファイル(`BalanceScreenView.swift` など)で足す。既存ファイルの変更は `RootView`(入口の1行)と `AppEnvironment`(`BalanceService` の生成。
  `.api` のとき `APIBalanceService`、`.mock` のとき `UnavailableBalanceService`)だけ。`project.pbxproj` はフォルダ同期なので原則触らない。

### 7. 範囲外
構築の保存(team-svc の責務)、技の検索シートの新規実装(既存 `MasterSearchSheet` の再利用を優先)、画像。

### 8. 第3段(P6-22): 仮想敵・おすすめタイプ・技範囲チェッカー
生成クライアント(`PokeCalcBalanceAPI`)は第1・2段で全操作を生成済みなので、再生成は要らない(契約は 0.7.0 の `threats`・`recommendations`・`move-range`)。

- **`BalanceService` に3操作を足す**(`threats(members:threats:)`・`recommendations(members:limit:)`・`moveRange(moveIds:)`)。
  `UnavailableBalanceService`(`balance_unavailable`)・`StubBalanceService`(テスト)・`APIBalanceService` も対応。
  threats/recommendations は `pokemonId`・`moveIds`・(あれば)`abilityId` を送る。`limit` は ViewModel からは渡さない(契約の既定 10。Web も同じ)。
  `move-range` は `moveIds` だけ(ポケモンを送らない。ADR-0404 §1)。倍率は応答の文字列のまま、`incoming`/`outgoing` の null は nil。
- **各機能は独立した世代カウンタ**(analyze・coverage に加え threats・recommendations・moveRange)。最新の呼び出しの応答だけを反映し、
  古い応答は成功でも失敗でも捨てる。1つの失敗・遅延は他を止めない。`cancel()`(画面を離れる)は全部の要求を cancel する。
- **呼ぶ条件(Web の ADR-0303 §7 と同じ)**: threats はメンバー1体以上かつ仮想敵1体以上のときだけ。recommendations はメンバー1体以上のときだけ
  (仮想敵は入力に含めない)。move-range は技1つ以上のときだけ(メンバーと無関係)。0体・0個では呼ばず結果を消す。
- **仮想敵の入力**: 最大 `TeamLimits.maxMembers`(6)体。メンバーと同じ `BalanceMember`・同じカード(`BalanceMemberCard` に `kind` を足して再利用)・
  同じ id 引きの操作(`setAbility`/`addMove`/`removeMove`。技は1体4つ・重複不可)。仮想敵の編集は threats だけ再計算し、
  メンバーの編集は analyze・coverage・threats(仮想敵があれば)を `refreshDebounce` で、recommendations を別の debounce で再計算する。
- **recommendations は重いので別の debounce(既定1秒)**。サーバーは同時計算数を絞り超えると `overloaded`(503。Retry-After。ADR-0409)を返すため、
  メンバーの編集ごとには呼ばず、操作が落ち着いてから1回だけ呼ぶ。Web はメンバー変更のたびに自動で呼ぶので、挙動はそれに合わせる
  (自動・独立・最新のみ)が、iOS は debounce を長くしたうえで、失敗時に「再計算」ボタン(ユーザー操作の再試行)を出す。
  ユーザー操作起点だけのロード(ボタンを押したときだけ呼ぶ)は、Web と挙動が分かれ、メンバー編集後に結果が古いまま残るので採らない。
- **技範囲チェッカー**: Web の画面には無い(API の TB6 だけ)ので、iOS は技を1〜4つ選ぶ UI を持つ(新規 UI。既存の `MoveSearchSheet` を再利用)。
  選択肢は直近の技検索の結果(先頭ページ・前方一致検索)から選択済みを除く(learnset は使わない=ポケモンを選ばない)。変化技だけの選択は
  サーバーが 400 `invalid_request`(「入力を見直してください」)にする。文言は iOS で決めた(§受け入れ条件 9。倍率の書式は設計 §10 と共通)。
- **候補・特性の表示**: おすすめの候補ポケモン・技範囲の「受けられるポケモン」は応答の `nameJa`(無ければ ID)を出す。特性名は契約に無い(abilityId だけ)ため、
  応答に出たポケモンの `species(key:)` から引く(Web の `abilitiesFor` と同じ考え方)。ただし引くのは応答の先頭
  `BalanceDisplayLimits.abilityNameResolveLimit`(40)匹まで・同じポケモンは1回だけ・失敗は無視(特性 ID のまま出し、画面のエラーにしない)。
  メンバー・仮想敵の特性候補からも名前を引く。長い一覧(受けられるポケモンは数百になりうる)は1リスト `BalanceDisplayLimits.pokemonPerList`(20)件までにして「ほか N件」。
- **表示**: 表は使わず縦並び(Dynamic Type 最大でも崩れない)。「安全」「抜群」は応答の真偽値を文字で出す(色だけに頼らない)。常時動くアニメーションは入れない。
  Web と同じ文言(仮想敵・おすすめの `balanceScreenText`・`balanceLabelText`)を `BalanceScreenText`/`BalanceLabels` に写す。モック構成では架空データを返さず、
  各セクションが「タイプバランスの API に接続できません」を出す。
- 却下: ViewModel を機能ごとに分割(メンバー・マスタ・検索の状態を共有するため同じ ViewModel に置き、第3段の stored property を足す)/
  仮想敵のカードを別 View に複製(`BalanceMemberCard` に `kind` を足して再利用)。

## 受け入れ条件
1. 契約: `analyze` は各メンバーの `pokemonId`(あれば `abilityId`)だけ、`coverage` は `pokemonId`・`moveIds` だけを `/api/balance/v1/team-balance/{analyze,coverage}` に POST し、全リクエストに `X-Device-Id`/`X-Session-Id` を付ける。応答の倍率(分数の文字列)・category・source・effect・null の `bestMultiplier` をそのままドメインに写す。
2. 倍率表示は設計 §10 の書式(「×4 弱点」「×2 弱点」「×1 等倍」「×1/2 耐性」「×1/4 耐性」「×0 無効」。攻撃範囲は「×2 抜群」「×1/2 いまひとつ」「攻撃技なし」)。語は応答の category から選び、倍率の値から判定し直さない。色だけで表さない。
3. メンバーは最大6体(超過は追加せず `tooManyMembers`・要求なし)、技は1体4つまで・重複不可、メンバー0体では balance を呼ばず結果を消す。構築(`Team`)から読み込める。
4. analyze と coverage は独立: 片方の失敗・遅延がもう片方を止めない。最新の呼び出しの応答だけを反映し、古い応答(成功・失敗とも)は捨てる。連続操作は debounce で最新の1要求にまとまり、画面を離れたら cancel して以後の応答を反映しない。
5. エラーは Web と同じ日本語に写す(英語の `message` を出さない)。通信失敗は「タイプバランスの API に接続できません」。モック構成では架空データを返さず同じ案内を出す。
6. `make ios-gen-check` が balance 生成物の一致も確かめる。`PokeCalcAPI` の生成物は変わらない。
7. (第3段)契約: threats は members・threats の `pokemonId`・`moveIds`・(あれば)`abilityId`、recommendations は members と(あれば)`limit`、move-range は `moveIds` だけを `/api/balance/v1/team-balance/{threats,recommendations}`・`/api/balance/v1/move-range/analyze` に POST し、全リクエストにヘッダーを付ける。応答の倍率・null・nameJa をそのままドメインに写し、エラー(400/413/422/503/500)・通信失敗・デコード失敗・キャンセルは第1・2段と同じ写像。
8. (第3段)threats はメンバー1体以上かつ仮想敵1体以上のときだけ、recommendations はメンバー1体以上のときだけ、move-range は技1つ以上のときだけ呼ぶ(0体・0個は呼ばず結果を消す)。仮想敵・技範囲の上限(仮想敵6体・技4つ・重複不可)を超える操作は要求を出さない。recommendations は仮想敵を含めない。
9. (第3段)3機能は独立した世代カウンタで stale 応答を捨て、1つの失敗が他を止めない。連続操作は debounce でまとまる(recommendations は専用の長い debounce)。画面を離れたら全要求を cancel する。文言は Web と同じ(仮想敵・おすすめ)。技範囲チェッカーの文言は iOS で決める。
10. (第3段)特性名は応答に出たポケモンから上限付きで引き、引けなければ ID のまま出す(エラーにしない)。長い一覧は上限で切って「ほか N件」。

## 人間確認が必要な点
- 実機・シミュレータでの見た目(Dynamic Type 最大、ダークモード、色以外で弱点が分かること)。Xcode の署名チーム・実機インストールは人間の作業(P6-4)。
- balance 0.8.0(ADR-0413)・gateway の balance 配線(ADR-0414。PR #478)が main に入った後の実機 E2E(クラスタ側の確認)。
