# ADR-0421: タイプバランスを構築画面に組み込む(別画面を廃止)

- 状態: 採用(ユーザー決定 2026-10-11。設計案はタイプバランスレーン。実装は 3 レーンに分担)
- 関連: ADR-0303(Web のタイプバランス画面)、ADR-0400(threats)、ADR-0401(recommendations)、ADR-0411(API 専用画面は常にオンラインのマスタ)、
  ADR-0415(iOS のタイプバランス画面)、ADR-0332(Web の構築編集 6 枠)、docs/type-balance-design.md、docs/usability-round3.md、
  ADR-0339(見た目の作り直し G-05)

## 背景
今は別画面でポケモンを 1 体ずつ追加してタイプバランスを見る。構築画面ですでに 6 匹を選ぶので手間が二重になる。
ユーザーの要望: 構築画面の中にタイプバランスの結果を出す。数匹だけ入れた状態でも判定できる。

## 決定

### 1. 入力と出力
- **入力は構築編集の 6 枠そのもの**(保存前の下書きでよい)。空の枠は無視し、**1 体からでも**結果を出す。
  別画面での追加操作・「構築から読み込む」は無くす。
- **出力は構築編集の中の「バランス」**(同じ画面のセクション。Web は 6 枠の下、iOS は構築編集の中のセクション)。
  内容は既存と同じ: 防御相性の表・チームの集計・攻撃範囲(coverage)・おすすめタイプ(recommendations)。
- **仮想敵(threats)は「バランス」内の任意の節**として残す(構築とは別の入力で、構築の枠は使わない。ADR-0400 のまま)。
  仮想敵の枠は保存しない(従来どおり画面の中だけ)。
- API・契約・サーバーは変えない(`analyze`・`coverage`・`threats`・`recommendations` を従来どおり ID だけで呼ぶ)。

### 2. 計算の呼び出し規則(挙動は ADR-0303 §9・ADR-0400・ADR-0401 を引き継ぐ)
- 送るのは「ポケモンを選んだ枠」だけ、枠の順。`abilityId` が無ければ省く。技は重複を 1 つにし、`coverage` は攻撃技が 1 つでもあるときだけ呼ぶ。
- 4 つの呼び出しは独立。1 つのエラー・遅延が他の表示を消さない。古い応答は無視し、入力が変わったら「計算中」に戻す。
- 入力の変更から 300ms 待ってから送る(1 キーごとに送らない)。ポケモンが 0 体なら呼ばず、結果の節は空の状態(一言)を出す。
- **balance が使えなくても構築の編集・保存は止めない**(絶対ルール 5 と同じ考え方)。エラーは「バランス」の節の中だけに role=alert で出す。
- 画面のマスタは構築編集のものを使う。balance の呼び出しは常にオンライン(ADR-0411)。

### 3. 部品の境界

| 層 | Web | iOS | 持ち主 |
|---|---|---|---|
| 入力の形 | `BalanceInputMember = {speciesKey, abilityId: string\|null, moveIds: string[]}`(`web/src/balance/types.ts`) | `BalanceViewModel.setParty(_ members: [Member])`(構築の下書き 1 体 = 1 要素。空は渡さない) | タイプバランス |
| 状態(呼び出し・キー・古い応答の無視・debounce) | `useTeamBalance(members, client, master, masterSearch)`(`web/src/balance/useTeamBalance.ts`) | `BalanceViewModel`(既存。`loadTeam` に加えて `setParty` を足す) | タイプバランス |
| 結果の表示 | `<BalancePanel state=… />`(`web/src/balance/BalancePanel.tsx`。表・集計・coverage・おすすめ・仮想敵の節) | `BalanceResultViews` / `BalanceThreatsView` / `BalanceRecommendationsView`(既存。セクションとして埋め込める形にする) | タイプバランス(中身)・Web / iOS(見た目は G-05) |
| 構築編集への組み込み | `TeamMemberEditor` が下書きから `BalanceInputMember[]` を作り `<BalancePanel>` を置く | `TeamEditView` が下書きから渡して `BalanceResultViews` を置く | Web / iOS |
| ナビ | タブ「バランス」を外す。`#balance`(旧 URL)は構築へ寄せる | タブ(または入口)を外す | Web / iOS |

- `BalanceScreen` は、`useTeamBalance` と `BalancePanel` を取り出した後に残るのは入力欄(メンバー・仮想敵の選択)だけになる。
  構築への組み込みが入ったら `BalanceScreen` と専用ルートを削除する(段階 4)。仮想敵の入力欄は `BalancePanel` の中の節に移す。
- 構築側はバランスの内部(キー・debounce・古い応答)を知らない。渡すのは `BalanceInputMember[]` だけ。

### 4. 段階と分担

| 段階 | 内容 | 担当 |
|---|---|---|
| 1 | Web: `useTeamBalance` と `BalancePanel` を `BalanceScreen` から取り出す(挙動・文言・テストの期待値は変えない) | タイプバランス |
| 1' | iOS: `BalanceViewModel.setParty` と、結果ビューを埋め込める形にする(既存の画面の挙動は変えない) | タイプバランス(iOS の画面ファイルの変更は最小。競合する場合は iOS レーンへ引き継ぐ) |
| 2 | Web: `TeamMemberEditor` に「バランス」を組み込む | Web |
| 2' | iOS: `TeamEditView` に組み込む | iOS |
| 3 | 見た目(G-05 の方針と ec の配色案) | Web / iOS |
| 4 | 旧タブ・旧画面(`BalanceScreen`・iOS の `BalanceScreenView` と入口)・旧 e2e の整理。`#balance` の寄せ先 | Web / iOS(Web が先) |

- 段階 1 が入るまで、Web の `BalanceScreen` の見た目の PR(I-web-13f)は止めてもらう(取り出しと同じ行を触るため競合する)。
  取り出し後は `BalancePanel` / 構築編集の中の「バランス」に対して見た目を作る。

### 5. 受け入れ条件
- 構築編集で 1 体選ぶだけで、防御相性・集計が出る(保存しなくてよい)。枠を外す・入れ替えると結果が追随する。
- 空の枠・技 0 個のメンバーでも落ちない(G-02 の再発防止)。
- balance が 503/タイムアウトでも、構築の編集・保存ができ、エラーは「バランス」の節の中だけ。
- ナビに「バランス」が無い。旧 URL は構築へ。仮想敵・おすすめ・coverage は従来と同じ結果。

## 代替案と不採用の理由
- 別画面のまま、構築から「読み込む」入口を強める: 手間(2 画面)が残り、ユーザーの要望に合わない。
- 保存済みの構築だけを入力にする: 数匹の下書きで判定したい要望に合わない。
- サーバー側で構築 ID を渡して分析する: balance が team に依存する(絶対ルール 4 に反する)。ID だけを渡す現行の形を保つ。

## 追記(2026-10-12): 段階 4(Web)完了
- `screens/BalanceScreen.tsx` と `screens/balance.screen.tsx` を削除し、画面 ID `balance` をレジストリ・ナビ・アイコン表から外した。旧 URL `/balance` は従来どおり構築(`/team`)へ寄せる(`app/routes.ts` の LEGACY_SEGMENTS。未知パスで計算になる挙動ではない)。
- 旧画面のテスト 4 ファイルは、`BalancePanel` / `useTeamBalance` / `MemberFields` の統合テストとして `web/src/balance/BalancePanel.integration*.test.tsx` に移した(件数・期待値は不変)。描画には、旧画面の入力欄をそのまま再現する `web/src/balance/balanceTestHarness.tsx`(テスト専用。アプリ本体からは import しない)を使う。
| テスト(移動先は web/src/balance/BalancePanel.integration*.test.tsx) | 件数 | 扱い |
|---|---|---|
| BalanceScreen.test.tsx → BalancePanel.integration.test.tsx(メンバー枠・analyze・coverage・仮想敵・threats・recommendations・エラー文言) | 全件 | 移動のみ。期待値は不変。描画対象を旧画面から balance/balanceTestHarness.tsx(同じ入力欄 + useTeamBalance + BalancePanel のテスト専用の足場。アプリ本体は import しない)に差し替え |
| BalanceScreen.online.test.tsx → BalancePanel.integration.online.test.tsx(capabilities・検索口・12 枠・coverage 判定) | 全件 | 同上 |
| BalanceScreen.g05.test.tsx → BalancePanel.integration.g05.test.tsx(カード・升目・CSS) | 全件 | 同上 |
| BalanceScreen.visual.test.tsx → BalancePanel.integration.visual.test.tsx | 全件 | 同上 |
| 削除したテスト | 0 件 | 旧画面固有(タブ・画面見出し・画面 ID)に依存するケースは元から無かった(全件がコンポーネント直描画) |

付け替え(期待値の最小変更):
- app/screenIcons.test.ts: `["balance","balance"]` の行を削除(画面 ID balance が無くなったため。Icon 名 balance は残るので Icon.test は不変)。
- app/screenRegistry.test.ts: 登録の表から balance の行を削除。hidden な画面の期待を `["balance"]` から `[]` に変え、balance が登録・SCREEN_ROUTES に無いことの確認を足した(弱めていない: 強めた)。
- app/routes.test.ts: `appText.balanceTabLabel` の確認を削除(語ごと未使用になったため ja.ts から削除)。`#balance` → team の寄せ先の確認は既存のまま(routes.test・App.routing.test)。
- images/PokemonIcon.placements.test.tsx・screens/visibleLabels.test.tsx・styles/inputTokens.test.ts: 描画対象を harness に替えただけ(inputTokens は BalancePanel.css を見ており不変)。
- e2e/balance.spec.ts は既に構築編集経由(変更なし)。
- 未使用になった `appText.balanceTabLabel` を削除。BalancePanel が使う文言(balanceScreenText 等)は残す。
- iOS 側の旧画面・入口の整理は iOS レーン(本 ADR §4 段階 4)。

## 追記(2026-10-21): 段階 4 の残り(テスト専用の足場 balanceTestHarness の撤去)
- `web/src/balance/balanceTestHarness.tsx`(旧画面の入力欄を再現していたテスト専用の足場)を削除した。統合テスト 4 ファイル(`BalancePanel.integration*.test.tsx`。ファイル名は BalancePanel・useTeamBalance・memberInputs と節の統合テストという内容に合うのでそのまま)は、実アプリの組み込み `team/TeamBalanceSection.tsx` を直接描画する。
- テスト共通の `web/src/test/balanceTestBed.tsx` は、メンバーの入力欄(MemberFields)で作った下書きを本物の節に渡すだけ(仮想敵・API 呼び出し・結果表示は本物)。アプリ本体からは import しない。仮想敵は本物が 0 体から始まるため、`renderBalanceTestBed` が最初の 1 枠を開く。
- 件数 116 → 116(`src/balance`)、全体の削除 0 件。期待値を変えたケースは次の 4 種(計 7 ケース)だけ:

| テスト | 変更 | 理由 |
|---|---|---|
| online「…画面ごと使えないことを案内する」×4 の「パーティ・仮想敵の入力は残すが、全部 disabled にする」 | メンバーは従来どおり disabled。仮想敵は「disabled」ではなく「入力欄・追加ボタンが出ない」を確認 | 本物の節は使えないとき仮想敵の入力ごと出さない(`available &&`) |
| recommendations「パーティが1体そろったら…limit 無しで送る」 | 素の `useTeamBalance + BalancePanel` を直接描画(期待値は不変) | 本物の節は limit 5 を足す(TeamScreen.balance B-5 が確認) |
| recommendations「候補の表: …該当ポケモンを応答の順で出す」 | 同上(期待値は不変) | 本物の節は該当ポケモンを選ぶタイルに替える。選ぶ口の無い表示は素の BalancePanel で確認 |
| g05「待機の状態に説明の段落(<p>)が無い」 | 素の `useTeamBalance + BalancePanel` を直接描画(期待値は不変: `p` が 0 件) | 本物の節はメンバー 0 体だとパネルを描画せず自分の案内の `<p>` を出すため、節越しではパネルの待機状態を確認できない |
- 注: online の 4 件はテスト名(「…全部 disabled にする」)を旧のまま残した(名前を変えると履歴で追えなくなるため)。仮想敵は期待値が「出ない」に変わっている。
- 差分(旧足場と本物の節): 仮想敵の削除は、旧足場は 1 体のとき不可、本物の節は常に可(`minCount` 0)。この差を検証するケースは元から無い。
