## 2026-09-24: iOS レーンの統合(PR #199)。issue #68 の残り(選択中の技 ID の名前解決)を getMove で解消、issue クローズ
Decision: ADR-0501「issue #68 の残り」のとおり、`PokeCalcService.move(id:)`(`getMove`)で**選択中の技だけ**を個別に解決する
(計算・逆算は1操作あたり最大4件、構築編集は保存済みメンバーの未知の技 ID を load 時に)。learnset 全件は解決しない
(一覧は「検索結果 ∩ learnset」のまま)。失敗時は従来の振る舞い(`moveUnavailable`/ID 表示)に戻す。持ち物の先頭ページが
検索上限200に達したら黙って切り捨てず案内を出す(Web は読み込みを中止するが、iOS は画面全体を止めない判断。ADR 参照)。
Reason: 他レーンのセッションから依頼(issue #68 の解消)。PR #136 の後に残っていた穴は、公開 API に技を ID で引く手段が
無いことが原因だったが、PR #161 の `getMove` で解消できた。critic 1周目 FAIL(逆算の古いエラー消去条件の退行)→修正→2周目 PASS。
Impact: issue #68 をクローズ。main に `getMovesByIds`(まとめ取り)が入ったので、構築編集の load は将来まとめ取りへ置き換え可能(任意)。
