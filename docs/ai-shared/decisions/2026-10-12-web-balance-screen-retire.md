## 2026-10-12: Web の旧タイプバランス画面(BalanceScreen)を撤去した(ADR-0421 段階 4)
Decision: BalanceScreen と画面の登録を削除。`/balance` は構築へ寄せる(既存)。旧画面のテストは削除せず `web/src/balance/BalancePanel.integration*.test.tsx` へ移し、旧画面の入力欄を再現するテスト専用の足場 `balanceTestHarness.tsx` に対して走らせる(件数 4165 → 4165、削除 0 件)。
Reason: 構築編集へのバランス統合(ADR-0421 段階 2・3)が済んでいるため。テストは絶対ルール 6 に従い消さず移した。
Impact: tb レーンの依頼(旧画面撤去)に対応。`useTeamBalance`・`BalancePanel`・`memberInputs` の API は不変。iOS の旧画面整理は iOS レーン。

移動・付け替えの一覧(ADR-0421 追記と同じ):

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
