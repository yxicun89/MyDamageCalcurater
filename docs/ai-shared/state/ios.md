## iOS
Lane: iOS(`ios/`。M3 の Phase 6。どの AI が進めてもよい)
Active: なし(P6-19〈PR #432〉・P6-7 完了。残る Next は他レーン待ちのみ)
Branch: feat/ios-p6(作業ディレクトリ ~/MyDamageCalcurater-ios)
Status: **M3(iPhone で使える)は完了**。P6-1(ADR-0500)・P6-2a 計算画面・契約追従・P6-2b 逆算画面・P6-2c 構築ビルダー
(一覧・編集・ニックネーム)・P6-2d(構築から個体を呼び出す配線)・P6-3・P6-4(手順書 `docs/runbooks/ios-device-install.md`)・
生成の internal タグ除外・DOC-ios は main に統合済み(PR #31・#53・#91・#119・#122)。
続けて Codex レビュー issue のうち iOS 主担当分を修正・main 統合済み: #100(種族変更後の特性ID残留)・#101(負のSPの
検証漏れ、PR #131)、#68(検索上限200件。種族・技ピッカーを `Menu` 一括取得から `.searchable()` 検索UIへ変更。
Web の ADR-0304 と同じ方針。PR #136)、#113(Web/iOS/API共同主担当。入力操作ごとの計算Taskを最新の1つだけ保持し
新入力・画面破棄で先行Taskをcancel、逆算の観測文字入力に200msのtrailing debounce。`CancellationError`は画面
エラーにしない。PR #166。1周目critic FAIL→2周目PASS)、#99(ライトテーマの danger コントラスト不足。
`ColorToken.danger`のライト値を`#E5484D`→`#CD1D23`に変更。Web PR #164 と同じ値。PR #170。issue #99 は
Web・iOS 両方完了でクローズ済み)、P6-6(issue #110の iOS側追従。ADR-0501「issue #110」章。`RequestLimits`/
`RequestLimitLabels` を新設し、観測16件・持ち物候補/比較64件(null込み。選べるのは63件)の上限を実装。
観測は追加ボタンを無効化、持ち物候補・比較トグルは上限到達中のON操作だけ拒否(OFFは常時可。Webの
決定的切り捨てとはあえて変えた判断はADR参照)。critic指摘でguardの位置(`beginInput()`より前)を固定する
回帰テストを追補。引き継ぎ検証で契約との同期検査 `ios/scripts/check-request-limits.sh` と観測上限の XCUITest を
追加。**PR #186 で main 統合済み**)。`make ios-test`(gen-check・件数上限の同期検査・XCTest 340件〈xcresult 集計353件〉・XCUITest 17件・Info.plist 検査)が緑。
issue #68 の残り(一度も検索結果に出ていない選択中の技IDを名前解決できない)は P6-9 で `getMove` による個別解決を
実装して解消(ADR-0501「issue #68 の残り」。持ち物の先頭ページが上限に達したら黙って切り捨てず案内を出す。critic
1周目 FAIL〈逆算の古いエラー消去条件の退行〉→修正→2周目 PASS。**PR #199 で main 統合済み、issue #68 クローズ済み**)。
issue #110 は API・データ・Web・iOS すべて完了したためクローズ済み(2026-09-24)。
P6-10(構築編集の load の技解決を `getMovesByIds` のまとめ取り1回へ。ADR-0501「getMovesByIds による構築編集の技の一括解決」。
critic 1周目 FAIL〈分割境界のテスト不足〉→テスト追加→2周目 PASS)完了。
P6-11(issue #334。攻撃側プリセットの表示名を技の分類に追従。PR #348、issue クローズ済み)・P6-12(issue #71 の iOS 追従。
`engine/presets/attacker.json` との契約テスト、並び 無振り→特化→振り、既定を無振りに変更。ADR-0501「P6-12」)完了。
P6-13(issue #274。計算画面の「詳細」: 急所・やけど・天候・フィールド・防御側の壁・攻撃側のランク・特性。PR #377。語は
DECISIONS.md に記録し Web が合わせる)・P6-14(最大の文字サイズで計算画面が横にはみ出す既存の不具合。結果行の `.fixedSize()` が原因)完了。
P6-15(アクセシビリティ域でプリセットのピルを縦積み等)・P6-16(issue #250。http は非修飾ホスト名と .local のみ受理、
NSAllowsLocalNetworking。PR #394/#395)・PR #372 追従の再生成(PR #398)・P6-17(未対応の印〈unsupported〉の表示。
文言は DECISIONS.md に記録し Web が合わせる)完了。
Next: (1) 第三者データの出典・非公式の表示(#328 のユーザー決定。文言は iOS が DECISIONS.md に既定案を書き Web が合わせる。Web と合意済み)。
(2) #272: API レーンが特性の契約(abilityId・unknownAbilityId・defenderOverride.abilityId)を出したら追従。
(3) API レーンが UnsupportedMark の reason/target を string に緩めたら、未知の値の扱いを追加(P5-4 の後に検討と連絡あり)。
(4) API レーンが `BulkCalcRequest.defenderOverride`(防御側のランク・特性・状態異常。
DECISIONS.md 2026-09-25 で採用、M2 の後に実装予定)を入れたら、iOS の「詳細」に防御側の入力を追加。
(5) P6-7(issue #103・ADR-0209 §8の削除UI)は完了(2026-10-01)。(1)〜(3) も完了済み(P6-18・P6-19・ADR-0215)。将来の候補:
engine の Champions マスタが pokedex-svc 経由になったら iOS のモック/実マスタの差し替え動作を再確認、Web の
record/team-svc(M2)が進んだら iOS の構築を端末内保存から API 保存へ移行するかを検討。
画面レジストリ化(P6-28・ADR-0507。2026-10-03): iOS に画面を足すときは `ios/PokeCalc/Features/<名前>Feature.swift` を書き、`FeatureRegistry.swift` に1行足す。`RootView`・`AppEnvironment` は編集しない。未マージのブランチの移行手順は ADR-0507。
