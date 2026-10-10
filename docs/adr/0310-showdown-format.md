# ADR-0310: Showdown 形式の変換部(web/src/team/showdownFormat.ts)

> 2026-10-11 追記: Showdown 形式の取り込み・書き出しは ADR-0342(G-03)で廃止した。画面・変換部のコードは削除済み。以下は履歴。

- 状態: 採用(2026-10-03。判定レーンの判断)
- 関連: ADR-0213 §4(Showdown 入出力はクライアント担当・team-svc に置かない)、ADR-0309(構築ビルダーの骨組み)、
  requirements.md §2、CLAUDE.md ドメイン規約(SP・名前をハードコードしない)

## 決定

`parseShowdownTeam(text, master)` と `exportShowdownTeam(members, master)` を、fetch・DOM・時刻・乱数を持たない
純粋関数として置く。返り値は `{members|text, issues}`。例外は投げず、問題は `ShowdownIssue`
(`severity` / `code` / `memberIndex`〈0 始まり。全体は null〉/ `field` / `value?`)に集める。

1. **`EVs:` は SP をそのまま読み書きする**。EV(0..252)から SP への換算は一意でなく推測になるため。
   範囲は 0..32・合計 66 以下。範囲外・負数・小数は `sp_out_of_range`(error)、合計超過は `sp_total_exceeded`(error)で、
   丸めずそのメンバーを取り込まない。32 超は EV らしい値として `ev_like_value`(warning)も添える。
2. **`ShowdownMaster`** は MasterData から名前引きに要る分だけの読み取り専用型(`nameEn` は任意)。MasterData はそのまま渡せる。
   名前は `nameEn`(trim・大小文字無視)と `nameJa` で引く。名前リストはコードに持たない。
3. **テラスタイプ**は API の `PokeType`(英語小文字)⇔ 大文字始まり(`Fire`)。スキーマ列挙の値で、マスタの名前リストではない。
4. **Level ≠ 50・IVs ≠ 31 は警告のみ**(`level_not_50` / `iv_not_31`。値は取り込まない)。出力でも Level・IVs 行は出さない。
5. **メンバーを落とす条件**: 種族が未解決、性格が未解決・欠落(`missing_nature`)、SP のエラー。持ち物・特性・技・テラスの
   未解決はその項目だけ省く(`unresolved_name`。技は詰める)。性別 `(F)` `(M)` は読んで捨てる。
6. 技は先頭 4 行だけ解決し、5 つ以上は `too_many_moves`、同一技は `duplicate_move`。解釈できない行は `malformed_line`。
7. **ニックネームが 24 コードポイント超**(ADR-0213 §3 の上限)は `nickname_too_long`(warning)で省く(既定案。切り詰めない)。
8. 出力で名前が無い ID は ID を出さずその項目だけ省き `missing_name`。種族・性格が無いメンバーは `error` で出力から外す。
9. 7 体目以降は取り込まず `too_many_members`。

## 帰結

画面(P5-5b)は `parseShowdownTeam` の `members` を構築に反映し `issues` を一覧表示する。性質テスト(export → parse の
200 パーティ)でラウンドトリップを保証する。

## 追記(critic 指摘の反映)

- **1行目の分解**: ニックネームは `@`・`(`・`)` を含みうる。`@` の位置と括弧の位置の候補(各最大8)を作り、種族が引けて持ち物も引ける
  候補を優先、なければ種族が引ける候補を選ぶ(種族名全体の引き当てが先)。名前が括弧を含む種族(`Mon (X)`)も引ける。
  持ち物・特性・技は行の残り全体を1つの名前として引くので、括弧入りの名前でも fallback は要らない。
- **名前の衝突**: 同じ名前(nameEn/nameJa・大小文字無視)が複数 ID に付く場合は先勝ちで解決し `ambiguous_name`(warning)を出す。
  export でも、その名前が元の ID に戻らない(別 ID が先勝ちになる)ときは `ambiguous_name` を出す(出力は行う)。
- **入力サイズ**: 全体が 100,000 文字超は `input_too_large`(error)で何も取り込まない。1行が 1000 文字超は `malformed_line`
  (値は先頭100文字)で捨てる(1行目なら、そのメンバーを入れない)。空白だらけの行の正規表現が遅くなるのを防ぐ。
- 同じラベル行(`EVs:` `IVs:` `Level:` `Ability:` `Tera Type:`)や性格行が2回出たら `duplicate_line`(warning)。先の行が勝つ。
- export で nickname が 24 コードポイント超なら `nickname_too_long`(warning)で省く。
- **非対応**: 全角括弧、フォルダ見出し行(`=== [gen9] Folder ===`)。`malformed_line` または種族未解決になる。
- issue の並びは「1行目 → ラベル行の出現順 → 性格の欠落 → 技」で、入力の行順に依存する。
