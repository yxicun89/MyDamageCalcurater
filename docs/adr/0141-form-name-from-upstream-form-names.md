# ADR-0141: 種族の姿の日本語名を、上流の姿の名前(form_name)から「基本種名（姿の名前）」で作る(issue #607)

- 状態: 採用
- 日付: 2026-10-09
- レーン: データ(importer・pokedex)。Web・iOS への影響なし(契約・表示は不変。`nameJa` の中身が日本語になるだけ)
- 関連: ADR-0140 §3(本 ADR で「地方の姿は生成しない」を改める)、ADR-0324(メガ種族の名前の生成。優先順位は不変)、
  ADR-0101 §6(日本語名の解決)・ADR-0103 §8(names の報告)、ADR-0136・0140(古い取得物の拒否)

## 背景(原因)

ADR-0140 は「上流(PokeAPI)に姿の日本語名が 1 件も無い」と測ったが、これは取得側の見落としだった。
`tools/importer/fetch-pokeapi.mjs` は `pokemon_form_names.csv` の **`pokemon_name` 列(完全名)だけ**を取り出していた。
この列は一部の姿(イワンコ（マイペース）など)にしか入っておらず、地方の姿・時間帯・天気・性別の姿は空。
一方、同じ CSV の **`form_name` 列(「ヒスイのすがた」「たそがれのすがた」「たいようのすがた」)には日本語(`ja`・`ja-Hrkt`)が入っていた**のに、
取得物に出していなかった。

## 決定

1. 取得: `fetch-pokeapi.mjs` は forms の各エントリに `formNames`(言語別の form_name)を足す(`names` は従来どおり完全名)。
   `formNames` は forms では必須(必ずオブジェクト)。取り込み(`DecodePokeAPISnapshot`)はキーの無い古い取得物を `ErrInvalidInput` で拒否し、
   `make import-fetch` を案内する(ADR-0136・0140 と同じ作法)。migration は不要(DB の列・`name_ja_source` の値は既存の `generated` を使う)。
2. 解決の優先順(メガ以外の姿。form != 0): 上書き > 上流の完全名(`names`。source `pokeapi`)> 姿の名前から生成(source `generated`)> 英語名(`fallback_en`)。
   生成は `master.FormNameJa(基本種の日本語名, 姿の名前)` = `基本種名（姿の名前）`(括弧は上流の完全名と同じ全角)。
   条件は、基本種の名前が `fallback_en` でない(上流か上書きにある)こと、姿の名前が空でないこと。姿の名前は `nameJaLanguages` の順で最初の空でないものを使う。
   名前・姿の一覧はコードに持たない(すべて取得元の form_name から導く)。
3. メガ種族(ADR-0324・0140)の優先順位・規則は変えない。メガは従来どおり `master.MegaNameJa`(フォーム名が `Mega`/`Mega-X`)。本規則は `is_mega` の行に適用しない
   (メガの名前の生成と二重にならないようにする)。
4. 規則の検証(推測で作らない根拠): 上流に完全名と姿の名前の両方がある姿では、取り込みのたびに「規則の結果 = 完全名」を確かめ、
   食い違えば警告 `name-form-rule-mismatch`(ID は showdownId)を出す。名前は上流の完全名を使い、止めない。
   現状の実データでは両方を持つ姿が無い(完全名のある姿は form_name が空)ため警告は出ない。規則の括弧は、その完全名の表記(全角）に合わせた。
   テストは架空データで、規則の結果と完全名の一致・不一致の両方を確かめる。
5. 作れない姿(基本種の名前が無い・姿の名前が空・Showdown の id と PokeAPI の slug が対応しない)は、推測せず英語名のまま
   `names.species.fallbackIds` に出す。上書き(`data/local/name_ja_overrides.json`)で補う。

## 結果(実データ。2026-10-09 のドライラン)

species の `fallbackIds`: 39 件 → 9 件。`generated` は 110 件(メガ + 姿)。残り 9 件は PokeAPI の slug が Showdown の id に対応しないもの
(`basculegionf`・`indeedeef`・`meowsticf`・`meowsticmmega`・`squawkabillyyellow`・`taurospaldeaaqua`・`taurospaldeablaze`・`taurospaldeacombat`・`vivillonicysnow`)。
slug を推測で対応づけると誤りうる(パルデアのケンタロスの3種は form_name が同じ「パルデアのすがた」)ため、上書きで補う。

## 影響

- 日本語名の見た目が変わる(英語 → 日本語)。API・契約・DB のスキーマは不変。反映は deploy-latest → 取り込み → master-release。
- ADR-0140 §3 の「地方の姿の生成は行わない」は、本 ADR で、上流の form_name から導く規則に置き換わった(括弧・接尾辞の訳を自前で持たない)。
