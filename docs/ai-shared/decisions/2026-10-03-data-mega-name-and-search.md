## 2026-10-03: メガ種族の日本語名を生成し、種族検索を「メガ + q」でも当てる(データレーン → Web・iOS レーン。issue #515・ADR-0324)
Decision: 上流に日本語名が無いメガ種族の `nameJa` は、importer が「メガ + 基本種の日本語名 + フォーム識別子(Mega-X → X)」から生成する
(`name_ja_source` = `generated`。migration 000011。基本種名が無いときは生成せず英語名のまま)。種族の名前検索は
「`nameJa` が q で始まる、または、メガ種族で `nameJa` が `メガ` + q で始まる」にそろえた(`q=ルカリオ` で基本種とメガの両方、`q=メガ` で全メガ)。
メガストーンの日本語名は機械的に作れないので生成せず、上書き設定(`name_ja_overrides.json` の items)と report の欠落一覧で扱う。
Reason: 取得元の日本語名がメガフォームで空のことがあり、「メガルカリオ」と入力しても当たらなかった(issue #515)。名前はクライアントで組み立てず importer が決める(検索・並びを全クライアントで揃える)。
Impact:
- **iOS レーン**: 検索は `GET /api/pokedex/species?q=` なので規則はそのまま届く。ローカルで種族を絞る箇所があれば ADR-0324 §3 の規則にそろえる。
  `SpeciesSummary` に `isMega` は無い(解決後の `SpeciesDetail` で分かる)。入力 UI の自動固定・UI テストは iOS レーンの残り(issue #515)
- **Web レーン**: オフライン検索(`createCachedOfflineMasterSource`)・テストの偽物・E2E フィクスチャは `web/src/master/speciesNameMatch.ts` の規則に変更済み。UI の固定は ADR-0320 のまま
- **運用**: 次の取り込みで `nameJa` が変わる(全行入れ直し)。実データの件数は import report の `names.species.generated`(`generatedIds`)で見る。`make master-release` で read model・calc に反映する
- migration 000011(species の CHECK に `generated`)を使う。down は `generated` を `fallback_en` に寄せてから戻す
