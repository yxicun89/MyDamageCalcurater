# ADR-0145: 効果定義の JSON にフィールドを足すときの互換方針 — 計算する側は厳格、read model の書き出しは寛容(ただし黙って落とさない)

- 状態: 採用
- 日付: 2026-10-11
- レーン: データ
- 関連: ADR-0118(effects.json 2つの一致)、ADR-0128(read model の metadata.json)、ADR-0105 §5(read model の形)、ADR-0176(TypeConvert 等の追加)

## 背景

balance・speed の Deployment は initContainer で、gitops overlay が digest で固定した pokedex イメージの `pokedex export` を動かして read model を作る。
DB の効果定義(abilities / items の effect JSON)に新しいフィールド(例 `TypeConvert`)が入ると、古いイメージの export が
`未知のフィールド "TypeConvert"` で失敗し、両 Pod が Init:Error になった。原因は、export が calc-svc・importer と同じ厳格なデコード
(`master.DecodeAbilityEffect`。未知のフィールドを拒否する)を使っていたこと。read model が使うのは防御側のタイプ相性の項目
(`DefImmuneTypes`・`DefAbsorbTypes`・`DefResistType`・`ReduceSuperEffective`)だけである。

## 決定

1. **厳格に読むのは計算する側**(calc-svc・WASM・importer)。同じリリースで一緒に更新するので、未知のフィールドはタイプミスとして捕まえる(従来どおり)。
2. **read model の書き出しは寛容**: `master.DecodeAbilityEffectIgnoringUnknown` を使い、効果定義のトップレベルにある、そのバイナリが知らないフィールドは読み捨てる。
   固定 digest の古いイメージが残っても export は落ちない。
3. **黙って落とさない**:
   - 読み捨てたフィールド名と、それを持つ特性の件数を `pokedex export` が標準エラーに警告として出す(`Report.IgnoredEffectFields`)。
   - **read model の JSON(metadata.json を含む全ファイル)のスキーマは変えない**。balance の read model ローダーと speed の `master/pokemon.go` は
     `DisallowUnknownFields` で厳格なため、印のキーを足すと却って両 Pod を落とす。JSON に印(例 `ignoredEffectFields`)を入れるのは、
     balance・speed のローダーを寛容にする PR が先に入ってからの後続とする。
4. **知っているフィールドは従来どおり厳格**: 型の不一致・必須の欠落・組の不整合・入れ子の未知のキー・同じタイプの無効と吸収の重複は export を失敗させる。許すのは未知のトップレベルのフィールドだけ。
5. 持ち物の効果定義は read model に出さないので影響しない。

## フィールドを足す PR のチェックリスト

- [ ] `effects.json`(2つ。ADR-0118)・`abilityEffectFields` / `itemEffectFields`・engine・ゴールデンを同じ PR で更新する。
- [ ] そのフィールドが read model(balance・speed)に影響するか書く。影響しない追加なら、固定 digest の更新は急がない(古いイメージは警告を出して動く)。
- [ ] read model の項目(防御側のタイプ相性など)を足す・変えるなら、`pokedex export` の出力と balance・speed のローダー・schema を揃え、**consumer(balance・speed の gitops overlay の pokedex イメージ digest)を先に更新**してから DB に新しい効果を入れる。
- [ ] デプロイ順: ① 新しい pokedex イメージを build して balance・speed の overlay の digest を上げる(古いイメージが警告つきで動くので急ぎではないが、警告を放置しない)→ ② マスタの取り込み(runbooks/data.md §4)→ ③ `make master-release`。
- [ ] `master-release` の後、initContainer のログに「読み捨てた」警告が出ていないか確認する。出ていれば、その consumer の digest が古い。

## 結果

- テスト: `services/internal/master/effects_immunity_test.go`(`TestDecodeAbilityEffectIgnoringUnknown`)、`services/pokedex/internal/readmodel/readmodel_immunity_test.go`(未知のフィールドがあっても出力のバイト列が同じ・壊れた既知の項目は失敗)。
- `readmodel_test.go` の「共通マスタの検査を通らない」ケースは入力を `{"Unknown": 1}` から既知の項目の型違い `{"StabMod": "x"}` に替えた(未知のフィールドを許すという意図した仕様変更のため。検査の強さは落としていない)。

## 限界

- 未知のフィールドがタイプ相性に関わるもの(将来、防御側の新しい項目)だった場合、古いイメージの read model にはその効果が出ない。警告ログでしか分からない。上のチェックリストの consumer 先行更新で防ぐ。
