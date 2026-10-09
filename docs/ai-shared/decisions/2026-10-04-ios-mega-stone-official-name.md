## 2026-10-04: iOS のメガストーンの表示名は正式名称を優先する(iOS レーン。ADR-0509 追記 §6')
Decision: メガストーンの `nameJa` が日本語の文字(ひらがな・カタカナ・長音・半角カナ・漢字)を1文字以上含むときはそのまま表示し、含まない(英語名のフォールバック)ときだけ従来の「{基本種名}のメガストーン」(基本種名が無ければ「メガストーン」)にする。判定は純粋関数 `ItemDisplayName.containsJapanese`、表示名は `ItemDisplayName.megaStoneName(for:baseSpeciesNameJa:)` の1関数に集約。ID 参照(要求・保存データ)は不変。
Reason: データレーンが `items.name_ja` に正式名称を入れた(日本語名の無い 40 件は英語名)。ユーザー要望「リザードナイトX のような正式名称が出てほしい」。ADR-0509 §6 の「nameJa は出さない」を更新した。
Impact:
- Web: `megaStoneLabel`・`itemsWithStoneLabels`(ADR-0326 §4)を同じ判定へ更新する別タスクが要る(データレーン依頼)。それまで Web は組み立てた名前のまま。
- モック: nameJa は「テスト」始まりに固定されるため、モックのストーンは日本語の正式名称扱い。英語名の分岐は単体テストのスタブで保つ。`MegaItemLockUITests` の期待値は反転(ADR-0509 追記の表)。
- 契約・API・services・Generated: 変更なし。
