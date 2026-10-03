# ADR-0506: 構築の「Showdown 風テキスト」は日本語名で書き、実 Showdown とは互換にしない

- 状態: 採用(2026-10-02。ユーザー決定。iOS レーン)
- 日付: 2026-10-02
- 関連: docs/plan.md P6-20、requirements.md §2(構築の Showdown 形式の取り込み・書き出し)、ADR-0213 §4(クライアント側の担当)、
  ADR-0500 §4、ADR-0501「P6-20」、docs/ai-shared/DECISIONS.md「nameEn は含めない」・同日「P6-20」

## 背景

requirements.md は構築の Showdown 形式のインポート/エクスポートを必須にしている。実 Showdown の形式は**英語名**で書かれるが、
iOS にも公開 API にも英語名(`nameEn`/`showdownId`)が無く、DECISIONS.md は「`nameEn` は含めない」と決めている。
この決定は覆さない。また Web の参照実装はリモートに無い。

## 決定

1. **日本語名の Showdown 風テキスト**にする。実 Showdown とは互換にしない(実 Showdown のテキストは貼っても名前が解決できず、
   「取り込めなかった行」に出る。黙って捨てない)。
2. **行構成は Showdown と同じ**(キーワードは英語のまま、値だけ日本語名):
   `名前 @ 持ち物`(ニックネームがあれば `ニックネーム (名前) @ 持ち物`)/ `Ability: 特性` / `Tera Type: タイプ` /
   `SP: 32 Atk / 20 Spe` / `Nature: 性格` / `- 技名`(4行まで)。メンバーは空行で区切る。書き出しの行の順は Showdown の出力順
   (先頭 → Ability → Tera Type → SP → Nature → 技)。取り込みは Ability 以降の順不同。
3. **SP は 0〜32・合計66**(EV ではない)。略称は Showdown と同じ HP/Atk/Def/SpA/SpD/Spe。`EVs:`・`IVs:` の行は
   「解釈できない行」(理由 `unsupportedStatLine`)として扱う。EV → SP の換算はしない(換算の根拠が無く、黙って値を変えないため)。
4. **名前の戦略を差し替え可能な型 `ShowdownNaming` にする**(既定は日本語名)。将来 API に `nameEn`/`showdownId` が足されたら、
   英語名の実装を足して差し替えるだけで英語名にも広げられる(その検索 API が要る場合も `searchQuery(forImportedName:)` で吸収する)。
   `api/openapi.yaml`・Generated は変えない。
5. **全か無かにしない**。取り込めなかった行は行番号・内容・理由を一覧で伝え、取り込めた分だけ追加するかを選ばせる
   (既定案・詳細は ADR-0501「P6-20」)。取り込みは保存を伴わない(`addMember` と同じ。保存は画面の「保存」)。
6. パーサ・シリアライザは PokeCalcCore の純粋関数、文言は Core の `ShowdownTextLabels` 1か所。上限(6体・4技・SP)は
   `TeamLimits`・`SPLimits`・`TeamValidator` を使い直書きしない。
7. **書き出しの名前解決の制約**。技は `moves(ids:)` のみで ID 引きでき、返らない ID は省く。補いの `searchMoves` 先頭ページ検索は
   best effort。持ち物は ID 引き API が無いため、書き出しは `searchItems` の先頭ページから名前を引き、先頭ページに無い持ち物は省いて
   利用者に件数で伝える(実データで起こりうる。省いた技・持ち物の件数と、種族を引けず飛ばした体数を書き出し結果の下に注意として出す)。
   将来 `getItemsByIds` 相当の API ができたら置き換える。
8. 取り込みの記号・数字は半角のみ。全角の `：`・全角数字は取り込めない行として報告する(全角空白 U+3000 は行頭・行末・`- ` の後で許容)。

## 却下した案

- 英語名の実 Showdown 互換: 英語名データが無い。API に足すのは API レーンの判断で、本タスクの範囲外。
- EV 形式の取り込み(EV → SP 換算): 換算規則が要件に無い。8SP = 64EV 等の近似を黙って入れると構築が意図とずれる。
- 取り込めない行が1つでもあれば全体を拒否: 実 Showdown の貼り付けは大半が名前不一致で、1行でも落ちると何も入らなくなる。
