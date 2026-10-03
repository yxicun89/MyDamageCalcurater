# ADR-0318: 「この端末のデータを削除」(P5-5d・issue #103 の Web 分)

- 状態: 採用
- 日付: 2026-10-02
- レーン: Web
- 関連: ADR-0209 §5・§7・§8、ADR-0314(情報ページ)、ADR-0308(タブの入力の保持)、ADR-0309(team クライアント)、
  ADR-0501「P6-7」と DECISIONS.md 2026-10-01「P6-7」(iOS の決定)、CLAUDE.md 絶対ルール5

## 決定(提案。implementer が緑にするテストが正)

1. **クライアント**: `web/src/record/recordClient.ts` を新設し(P5-5c の同名ファイルと型名・`request` の流儀を揃える)
   `deleteDeviceData()` を足す。`teamClient.ts` にも `deleteDeviceData()` を足す。どちらも `DELETE`・本文なし・
   `X-Device-Id`/`X-Session-Id` のみ、例外を投げず `Result` で返す。200 でも `status` が `completed|partial` でなければ
   `*_unavailable`。1回の呼び出しは1回の HTTP 要求(繰り返しは呼び出し側)。
2. **手順は純粋関数**: `web/src/deviceData/deleteDeviceData.ts` の `runDeviceDataDeletion`。iOS の ViewModel と同じ規則:
   record と team を独立に呼ぶ(片方の失敗・未完了でももう片方は進める)/ `partial` は対象ごとに最大 20 回まで繰り返し、
   超えたら `incomplete`(失敗ではない)/ 通信エラーは自動再送せず `failed(code)` / `targets`・`previous` で再試行
   (completed でない対象だけ。上限は数え直す)/ `AbortSignal` で中断(以後送らず `pending`)。
   `describeDeviceDataDeletion` が結果を ADR-0209 §8 の文言に直す(サーバーの message は出さない)。
3. **置き場所**: 情報ページ(`AboutScreen`)に「データの扱い」節(h3)。新しい画面・導線は作らない(iOS と同じ)。
   props で `recordClient`・`teamClient`(`deleteDeviceData` だけを要る型)と `onTeamDataDeleted` を受ける。
4. **ローカル状態は消さない**(iOS と同じ): 消すのはサーバーの record / team だけ。端末 ID(`pokecalc.deviceId`)も計算モード等の
   localStorage も触らない。端末 ID を作り直すと削除後も「同じ端末」としての墓石(ADR-0209 §7)が効かず、
   以後の操作が別端末として扱われて混乱するだけで利点が無い。
5. **文言**: `i18n/ja.ts` の `deviceDataText` に1か所。ADR-0209 §8 を完全一致で使い、説明2文目の括弧だけ Web 向け
   (「ブラウザのサイトデータを消したとき」)。§8 に無い4文目「削除するのはサーバーに保存したデータだけです。計算・逆算は、
   削除の成否にかかわらず使えます。」を Web 側の補足として足す(何が消え何が消えないか・絶対ルール5。人間レビューで文を直してよい)。
6. **構築一覧の再取得**: `TeamScreen` に optional の `reloadToken`。App が `onTeamDataDeleted` で数値を進めて渡し、
   マウント後に変わったら `list()` を呼び直す(読み込み中表示に戻し、最新の呼び出しの応答だけ採る)。team が completed になったら
   record が失敗していても合図する(再試行の record だけの run では合図しない)。
7. **確認ダイアログ**: `window.confirm` は使わず、アプリ内の `role="alertdialog"`・`aria-modal`。名前は「この端末のデータを削除」、
   説明は確認文。初期フォーカスはキャンセル、Esc とキャンセルで閉じて削除ボタンへフォーカスを戻し、Tab はダイアログ内で循環。
   実行中は削除ボタンを無効にする(二重起動しない)。進行・完了は `role="status"`、失敗は `role="alert"`。
8. **オフライン・API 不達**: 削除は常にサーバー操作なので、不達は「サーバーに届きませんでした。…」で失敗表示し、黙って成功にしない。
   計算は影響されない(オフライン WASM モードでも画面は使える)。

## 検討した代替

- 完了後に端末 ID を再生成する: 上記 4 のとおり利点が無いため見送り(iOS も作り直さない)。
- `window.confirm`: 破壊的操作のフォーカス・読み上げ・デザインを制御できないため見送り。
- `partial` を手動再試行だけにする: ADR-0209 §8 は自動再送でもよいとしており、iOS と同じ自動(上限付き)を選ぶ。

## 結果

実装済み(2026-10-02)。`web/src/record/recordClient.ts`(P5-5c の同名ファイルと型名・書き方を揃えた。`listFrequentOpponents` は
5c で足される)・`teamClient.deleteDeviceData`(`TeamClient` 本体は変えず `TeamDeviceDataClient` を `createTeamClient` の戻り値に足す。
既存の TeamScreen テストの fake を壊さないため)・`deviceData/deleteDeviceData.ts`・`deviceData/DeviceDataSection.tsx`・
`AboutScreen`(クライアントと合図は optional props。3つ揃ったときだけ節を出す)・`TeamScreen.reloadToken`(App が `onTeamDataDeleted` で
進める)。既存の `App.about.test.tsx` の見出し構造は h3「データの扱い」の分だけ更新した。web の vitest 1998件・typecheck・lint・
`make web-e2e` 49件・`make check-publishable` green。テスト: `src/record/recordClient.test.ts`・`src/team/teamClient.deviceData.test.ts`・
`src/deviceData/deleteDeviceData.test.ts`・`src/AboutScreen.deviceData.test.tsx`・`src/team/TeamScreen.reload.test.tsx`・
`src/App.deviceData.test.tsx`・`src/i18n/deviceDataText.test.ts`・`e2e/deviceData.spec.ts`。

追記(2026-10-02。critic 推奨の反映):
- 画面を離れて中断した場合は以後の要求を送らない(一部が残りうる)。ただし team が completed になっていれば、中断後でも
  `onTeamDataDeleted` を呼ぶ(setState だけで unmount 後も無害)。
- 削除の実行は `runningRef` で同期的に二重起動を防ぐ(同一フレームの2回押しでも1回だけ)。
- 統合メモ(P5-5c = feat/web-record-frequent-p5-5c): `recordClient.ts` の add/add 衝突は 5c 側に `deviceData`/`deleteDeviceData` を足して解消、
  `ja.ts` の `recordClientText` の重複は片方に統合、`App.tsx` の `createRecordClient` 生成は1つにする。
- 既知: `TeamScreen` の reload 後も、編集中の下書き(名前変更・新規作成フォーム)は残る。
