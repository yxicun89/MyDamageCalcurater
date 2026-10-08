## 2026-10-09: 種族の姿の日本語名を上流の form_name から「基本種名（姿の名前）」で作る(issue #607 の残り。ADR-0141)
Decision:
- 原因は取得側の見落とし。PokeAPI の `pokemon_form_names.csv` の `form_name`(「ヒスイのすがた」など)を捨て、空の `pokemon_name` だけを見ていた。
  取得物の forms に `formNames` を足し(必須。古い取得物は拒否して `make import-fetch` を案内)、取り込みで基本種名 + 全角括弧 + 姿の名前を生成する(source `generated`)。
- 優先順: 上書き > 上流の完全名 > 生成 > 英語名。メガは従来どおり(ADR-0324 の優先順位を変えない)。完全名がある姿では規則の結果との一致を確かめ、食い違えば警告 `name-form-rule-mismatch`。
- 実データで species の fallbackIds は 39 件 → 9 件。残りは PokeAPI の slug と Showdown の id が対応しない姿で、上書きで補う。
Reason: 上流にデータがあるのに使っていなかった。名前の表・括弧の規則をコードに持たず、取得元から導く(CLAUDE.md「ハードコードしない」)。
Impact: Web・iOS への影響なし(契約・表示は不変。`nameJa` が日本語になるだけ)。反映は deploy-latest → 取り込み → master-release。
