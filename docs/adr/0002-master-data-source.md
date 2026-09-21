# ADR-0002: マスタデータ(使用可能ポケモン・技・持ち物・特性)の取得元

- 状態: 暫定(人間の確認待ち)
- 日付: 2026-09-21
- 関連: P2-1(plan.md)、P2-2(importer)、P2-3(pokedex-svc)、docs/requirements.md「データモデル」「逆算の持ち物候補」、
  ADR-0005(補正定義はマスタから)、CLAUDE.md ドメイン規約(SP・ハードコード禁止)、docs/test-strategy.md(ゴールデン)
- 番号について: 0002 は初期構想時に確保されていた欠番で、P2-1 の調査結果として本 ADR が埋める

## 背景

P2-2 で、ポケモン・技・持ち物・特性・性格・タイプ相性を MySQL に取り込む importer(k8s CronJob)を作る。
その前に「チャンピオンズで使用可能な集合」と各種数値・日本語名の取得元を決める必要がある。
現在のゴールデンは @smogon/calc 0.10.0 の gen9 参考集合(1392 種、CAP を含み、チャンピオンズの使用可否とは無関係)で代用している。

調査日はすべて 2026-09-21。**事実**(出典と確認方法を付記)と**未確認**を分けて書く。

### 確認方法の区別(信頼度)

- **[ローカル検証]**: npm パッケージと Git リポジトリを作業用の一時領域に取得し、コードを実行して確認した。
  リポジトリ(pokecalc)にはデータを取り込んでいない。信頼度が最も高い。
- **[取得]**: WebFetch で URL を取得し、内容を要約させたもの。要約モデルを経由するため、数値・向きの取り違えがありうる
  (実際に §調査結果の Snap Trap で他ソースと食い違う要約が出ている)。
- **[検索]**: 検索結果のスニペットのみ。ページ本体は取得していない。根拠としては弱いので採用判断に使わない。

## 調査結果

### 1. ゲームの前提(事実)

| 項目 | 内容 | 出典・確認 |
|---|---|---|
| 存在・発売日 | Nintendo Switch 2026-04-08、モバイル 2026-06-17(Bulbapedia の記述) | [取得] https://bulbapedia.bulbagarden.net/wiki/Pok%C3%A9mon_Champions |
| レベル・個体値 | 表示されないが Lv50 として計算。個体値は 31 固定で変更不可 | [取得] 同上 |
| 能力ポイント(SP) | 1 ステータス最大 32、合計 66。HP = `Base+SP+75`、他 = `⌊(Base+SP+20)×性格補正⌋`(補正 1.1 / 0.9 / 1) | [取得] https://bulbapedia.bulbagarden.net/wiki/Stat_point |
| SP と努力値 | HOME からの移行は「最初の 1 ポイントが 4 EV、以降 1 ポイント 8 EV」= `8×SP−4` | [取得] 同上。Showdown の champions mod のコメントも同じ([ローカル検証]) |
| 式の実装 | Showdown champions mod(`statModify`)と @smogon/calc 0.12.0 の Champions 世代(`calcStatChampions`)が上記の式で実装済み | [ローカル検証] |
| メガシンカ | 1 回の対戦で 1 度。60 種以上がメガシンカ可能(公式)。Serebii の使用可能ポケモン表はメガをフォーム別の行として載せる。A/B のデータでもメガは別の種族エントリ(82 件) | [取得] https://news.pokemon-home.com/ja/page/751.html 、https://www.serebii.net/pokemonchampions/pokemon.shtml。A/B は [ローカル検証] |
| 使用可能集合はレギュレーション制 | M-A(2026-04-08〜06-17、213 匹・姿違い含む・基本は最終進化+ピカチュウ)、M-B、M-C(2026-09-08〜12-01)と入れ替わる。M-C の追加は公式記事で「新たに使用可能なポケモン 24 匹」+メガ 6 種(他の記事は 29 と書き、数え方が揃わない)。過去のレギュレーションで使えたものも引き続き使える(公式) | [取得] https://www.gamespark.jp/article/2026/04/08/164904.html 、https://www.pokemon.com/us/news/get-ready-for-regulation-set-m-c-in-pokemon-champions |
| 公式の完全な一覧 | 公式記事が示す一覧の場所は**ゲーム内**(Recruit → Roster Info)。公式 Web で全種・全持ち物・全技を機械可読に公開しているページは**確認できなかった** | [取得] 同 pokemon.com 記事。Web 上の公式一覧の不在は「見つけられなかった」であり、存在しないとは断定しない |
| 技の選び方 | レベルアップ・遺伝・わざマシンではなくリストから選択する。使用不可の技を持つと編成時にエラー | [取得] https://www.serebii.net/pokemonchampions/moves.shtml 、https://bulbapedia.bulbagarden.net/wiki/List_of_moves_by_availability_in_Pok%C3%A9mon_Champions |

### 2. 取得元の候補(比較)

| 候補 | 取れるもの | 日本語名 | 形式 | 更新の実績 | ライセンス・規約 | Champions 対応 |
|---|---|---|---|---|---|---|
| **A. @smogon/calc 0.12.0**(npm) | 種族(タイプ・種族値・特性スロット)、技(威力・タイプ・分類・優先度)、持ち物名、特性名、性格、タイプ相性。**Champions 世代(`Generations.get(0)`)に使用可能集合と SP 式を内蔵** | なし | TS/JS のデータ(コード内リスト) | 0.12.0 は npm の modified が 2026-09-18 | MIT(npm の license フィールドと GitHub の記述)。ただしデータの元は任天堂・ゲームフリークの知的財産(MIT はその権利を与えない) | あり([ローカル検証])。ただし GitHub の README は Champions に言及せず、リリースページは空 [取得] |
| **B. Pokémon Showdown**(`data/mods/champions`、`championsregmb`) | 使用可否(`isNonstandard`)、技の変更(威力・PP・タイプ)、持ち物の可否、習得技(`learnsets.ts`)、メガ石との対応(`requiredItem`)、SP 式 | なし | TS | 取得時点の HEAD は 2026-09-20(commit f10d679)。Reg M-C 用が `champions`、Reg M-B 用が `championsregmb` | MIT(LICENSE を [取得])。データの権利は A と同じ扱い | あり。ただし species/ability の数値は gen9 を継承する(下記) |
| **C. PokeAPI** | 図鑑番号、日本語名(ja / ja-hrkt)、Champions 図鑑(pokedex 36 = 231 種)、Champions の習得技(version-group `champions`)、フォーム一覧 | あり([ローカル検証]で「ガブリアス」「こだわりハチマキ」「きりさく」を確認) | REST/JSON、CSV/api-data リポジトリ | 最近のバージョングループを追従 | コードは BSD-3-Clause。データ自体の利用条件は README に明記なし [取得]。公平利用: 「取得したものはローカルにキャッシュせよ」「レート制限は撤廃したが頻度を抑えよ」[取得]。自前ホスト用の CSV あり | 部分的(§3) |
| D. コミュニティ製データセット(GitHub) | 種族・技・持ち物・特性・習得技(258 キャラ、技 900、持ち物 583 など。**使用可能フラグは確認できず**) | 未確認(記述なし) | JSON | otterlyclueless/… は最終 push 2026-04-16 で古い。vbbjandrade/… は fork で 2026-09-18 に更新 | CC BY 4.0 を名乗るが、GitHub の license 判定は NOASSERTION。元データは Showdown 等 [取得] | 名目上あり。**出所が A/B の再配布**で、一次情報としての価値は低い |
| E. 攻略サイト(Serebii、GameWith、Game8、やっくん 等) | 一覧表。使用可能技・持ち物・種族の表 | Serebii の当該表に日本語名なし。日本語サイトは日本語あり | HTML | 随時 | スクレイピングは規約が未確認。やっくん(yakkun.com)は自動取得で 403 を返した(回避しない) | あり |
| F. Bulbapedia | 使用可否の表(✓/✘)、SP・式の解説 | 一部 | HTML(wiki) | 随時 | **CC BY-NC-SA 2.5(非営利・継承)** [取得]。robots.txt は `Crawl-delay: 5`([ローカル取得]) | あり |
| G. ゲームデータの解析 | 一次情報 | ― | ― | ― | **未調査**。適法性・規約上の懸念から対象外とする | ― |

### 3. 候補 A・B・C の突き合わせ結果([ローカル検証])

作業用の一時領域で、`@smogon/calc@0.12.0` と `smogon/pokemon-showdown`(commit f10d679、2026-09-20)を取得して集計した。

| 項目 | @smogon/calc 0.12.0(Champions) | Showdown `champions`(Reg M-C) | 備考 |
|---|---|---|---|
| 種族(フォーム込み) | 359(通常 277 + メガ 82、基底種 231) | 392(通常 310 + メガ 82、基底種 237) | 差は calc 側のみ 2(Aegislash-Both / Aegislash-Shield)、Showdown 側のみ 35(Vivillon の模様違い、Meloetta-Pirouette など。見た目違いが中心だが、全件の内訳は未確認) |
| 技 | 526(番兵 `(No Move)` を含む) | 515 | calc のみ 11(Anchor Shot、Blood Moon 等。下記)、Showdown のみ 1(Pound) |
| 持ち物 | 166 | 166 | **集合が完全一致** |
| 特性(リスト) | 215 | 316(可否フラグは少数のみ) | 特性は種族の特性スロットから引くのが確実 |
| Reg M-B(`championsregmb`) | ― | 種族 357、持ち物 148、技 515 | M-C は M-B より種族 35・持ち物 18 多い。M-B の種族は M-C の部分集合 |

- **PokeAPI の Champions 図鑑(pokedex 36)は 231 種**で、calc の基底種 231 と一致した(Aegislash は `aegislash` / `aegislash-blade`、Floette は `floette` / `floette-eternal` の表記違いのみ)。
- 公式・準公式の件数: Reg M-A 213 匹(姿違い含む)。GameWith は「全 352 種(通常 270 + メガ 82)」と書く [取得]。
  数え方(フォームの扱い)がソースごとに違うため、**件数の一致は判断材料にならない**。集合そのもの(ID 単位)で照合する必要がある。
- **種族値・タイプ・特性は gen9(SV)から変わっていない**(Showdown の champions mod は `pokedex.ts` を持たず、gen9 との差分は 0 件。calc の Champions 種族も SV データの部分集合)。
  コミュニティのデータセットは「種族値・特性が変わっているかもしれない」と書くが、**変更の実例は見つけられなかった**(未確認)。
- 技の Champions 固有変更(威力・タイプ)は、A と B で次が一致した(gen9 → Champions。英語名で記す。日本語名は importer が PokeAPI から引く):
  Snap Trap のタイプ Grass → Steel、威力 = Apple Acid 80→90、Beak Blast 100→120、Bone Rush 25→30、Fire Lash 80→90、First Impression 90→100、
  Grav Apple 80→90、Infernal Parade 60→65、Meteor Assault 150→170、Mountain Gale 100→120、Night Daze 85→90、Psyshield Bash 70→90、
  Slash 70→80、Snipe Shot 80→85、Spirit Shackle 80→90、Trop Kick 70→85。
  A と B で**一致しない**もの: Growth のタイプ(Showdown は Normal → Grass、calc は Normal のまま。変化技なのでダメージには影響しない)、
  命中の変更(Crabhammer、Make It Rain、Syrup Bomb 等)は Showdown にあり calc には無い(ダメージ量には影響しない)。
  Serebii の「Updated Attacks」ページ [取得] は、上の威力の変更(Slash 70→80 等)を含み、Growth も Grass へ変更としている。
  **食い違い**: 同ページの要約は Snap Trap を「Steel→Grass」と書き、A/B(gen9 の Grass → Steel)と向きが逆に読める。
  要約の誤りの可能性が高いが、原文は未確認。ダメージに効くため、ゲーム内で確認する(人間の確認事項)。
  PokeAPI の技データ(例: きりさく)は 70 のままで、Champions の変更を反映していないことを確認した(**Champions の技数値の正には使えない**)。
- 使用可否について A と B が食い違う技: calc のみが Champions の技として持つ 11 技(Anchor Shot、Astral Barrage、Blood Moon、Bolt Beak、Dragon Hammer、
  Fishious Rend、Gear Grind、Hyper Drill、Metal Claw、Revelation Dance、Triple Dive)は、Showdown では gen9 由来の「過去作限定」扱いのまま(Pound は逆に Showdown のみ)。
  Showdown 側にも一部に Champions 用の威力の上書き(Anchor Shot 90 等)があるため、どちらかが取りこぼしている可能性がある。**未確定**。

### 4. 持ち物(CLAUDE.md / requirements の例と食い違う可能性)

A・B の持ち物集合(166 件、完全一致)には、**こだわりハチマキ、こだわりメガネ、とつげきチョッキ、しんかのきせき(Eviolite)が含まれない**。
Serebii の持ち物一覧ページ [取得] の要約も、この 4 件を「見つからない」としており、3 ソースが一致した。
一方、Choice Scarf、Life Orb、Expert Belt、Muscle Band、Wise Glasses、タイプ強化系(Charcoal、Mystic Water 等)、半減きのみ(Occa、Passho、Yache 等)、
Rocky Helmet、Leftovers、Focus Sash、Sitrus Berry、Lum Berry、メガストーンは含まれる(calc の一覧で確認)。

- docs/requirements.md の逆算の持ち物候補(「攻撃側: こだわり系」「防御側: 特防・防御を上げる持ち物」)と ADR-0005 の例(こだわり系・とつげきチョッキ)は、
  **現行のレギュレーション(M-C 相当)では使用不可の持ち物を例に挙げている**。
  「マスタから使用可能なものを自動抽出する」設計なので機構は影響を受けないが、「攻撃側: こだわり系」の分類は現状では空になり、
  「防御側: 特防・防御を上げる持ち物」も該当が無い可能性がある(分類の定義とマスタの突き合わせは P2-2 で確認する)。
- ゴールデン生成器の効果アダプタ(こだわりハチマキ、しんかのきせき 等)は、エンジンの**計算式の検証**が目的で、使用可否とは独立。ここは維持するのが妥当と考える(§決定 7)。

### 5. 特性

Champions 専用の特性が calc に 7 件ある(Aura Guard、Dragonize、Eelevate、Fire Mane、Mega Sol、Piercing Drill、Spicy Spray)。
PokeAPI は Dragonize(ドラゴンスキン)、Mega Sol(メガソーラー)、Piercing Drill(かんつうドリル)の日本語名を持つが、Fire Mane と Eelevate は日本語名が空だった。
つまり **PokeAPI の日本語名は新規の特性・メガ石・メガフォームで欠落がある**。Clefablite・Chesnaughtite のメガ石も日本語名が空、
`clefable-mega` / `floette-mega` は `pokemon` エンドポイントに存在する。フォームの日本語名は `pokemon-form` 側にあるはずだが**未確認**([ローカル検証])。

### 6. ヌケニン(Shedinja)

- calc 0.12.0 の Champions 種族にも、Showdown champions mod にも(`isNonstandard: "Past"`)、PokeAPI の Champions 図鑑にも**ヌケニンもヌケッチャ(Nincada)も含まれない**([ローカル検証])。
- 一方、GameWith(gamewith.ai)にはヌケニンの個別ページがあり HP 種族値 1 と表示される [取得]。攻略サイトの汎用データの可能性があり、使用可否の根拠にはならない。
  やっくん(yakkun.com)の Champions 用ヌケニンのページは自動取得で 403 のため確認できなかった。
- calc の Champions 用 HP 式は `base === 1 ? 1 : base + sp + 75` で、種族値 1 の HP=1 特例を保持している(Bulbapedia の SP 記事は、ヌケニンには触れていない)。
- したがって「ヌケニンを HP=76 とするか」は、**使用可能集合にヌケニンがいない限り論点にならない**。現時点で使用可能とする根拠は得られなかった(未確認)。

### 7. 日本語名と ID の対応

- 日本語名は A・B に無い。**PokeAPI が唯一の機械可読な取得元**(species / move / item / ability に `ja` と `ja-hrkt`)。新規・Champions 専用の名称には欠落がある(§5)。
- 図鑑番号: PokeAPI の `pokemon-species` の id が全国図鑑番号(ガブリアス = 445)。フォームは `pokemon` の id が 10000 番台(`garchomp-mega` = 10058)で、
  `varieties` に `is_default` の順序がある。**`{図鑑番号4桁}-{フォルム3桁}` のフォルム番号を定義している外部ソースは無い**ので、importer 側でフォルム番号を採番する必要がある。
- A(名前、例 `Garchomp-Mega-Z`)、B(小文字英数のみの ID、`garchompmegaz`)、C(スラッグ、`garchomp-mega-z`)は、小文字英数のみに正規化すれば結合できる(`Garchomp-Mega-Z` の実在を A・C で確認)。

### 8. 補正定義(ItemEffect / AbilityEffect)の取得元

A・B・C のいずれも、**4096 基準の補正値を構造化データとして持っていない**。A は計算ロジックがコード内の分岐、B も効果はイベントハンドラのコード、C は効果の説明文のみ。
ADR-0005 の「補正定義はマスタから解決する」を満たすには、**importer が持つ、人が管理する補正定義ファイル**(持ち物ID・特性ID → ItemEffect/AbilityEffect)が必要になる。
外部ソースは「どの持ち物が使えるか」と名前を提供し、補正値は自前で持つ。

## 決定

暫定。§人間の確認事項の回答で変わりうる。

### 1. 使用可能集合と数値の正: @smogon/calc 0.12.0 の Champions 世代を機械可読の一次ソースとする

- 種族・技・持ち物・特性・性格・タイプ相性は、**pin した @smogon/calc(0.12.0)の Champions 世代**から生成する。
- **Showdown champions mod は照合用(クロスチェック)と、A に無い情報(習得技、メガ石 ↔ メガフォームの対応 `requiredItem`)の取得元**とする。
  A と B の集合が一致しない項目(§3 の技の食い違い、種族の差)は、importer が差分を報告し、人間が裁定した結果を補完ファイルに書く。
- **人間による確認基準は「ゲーム内の Roster Info と、実機で選べる持ち物・技」**(公式の完全な一覧がゲーム内にしか無いため)。A・B の一致は「他人の集計が一致した」以上の保証ではない。

### 2. 日本語名: PokeAPI から取得し、欠落は補完ファイルで持つ

- ja(なければ ja-hrkt)を PokeAPI から取得。欠落分(§5)は、リポジトリで管理する補完ファイル(名称 ID → 日本語名)で人が埋める。
- 補完が無い項目は英語名にフォールバックし、importer が欠落件数を報告する(画像と同様に「無くても成立する」方針)。

### 3. 取得方式: スナップショットをコミットし、importer は外部に取りに行かない

- 「スナップショット生成」(Node。`tools/golden` と同様に pin した npm・Git の版から正規化 JSON を作る手動ツール)と、
  「取込」(Go の importer。コミット済みのスナップショットを読み、MySQL へ冪等に upsert)を分ける。
- CronJob の importer は**実行時にネットワークへ出ない**。スナップショットの版(チェックサム)と `data_versions` を比較し、変更があるときだけ取り込む冪等な照合として動かす。
  レギュレーションの入れ替えは約 2〜3 か月ごとで、頻繁な自動取得は不要。
- スナップショットの更新は人が `make` ターゲットで行い、差分をレビューしてコミットする。

### 4. レギュレーションの扱い

- 使用可能集合はレギュレーション(M-A/M-B/M-C…)で変わり、**過去に使えたものは引き続き使える(累積)**ため、現行の集合は過去の集合の上位集合。
- マスタには `available`(現行のレギュレーションで使用可能か)を持たせ、データの版に `regulation`(例 `M-C`)を記録する。
  過去のレギュレーションの集合は持たない(必要になったら追加)。

### 5. ゴールデンの種族集合の差し替え(P2-1 の子項目)

- 次の手順を推奨する。**本 ADR では実施しない**(コード変更・再生成は P2 内の別タスク):
  1. `tools/golden` の @smogon/calc を 0.12.0 に上げ、`Generations.get(0)` を使う。SP をそのまま渡せる(`8×SP−4` の換算が不要になる)。
  2. 種族集合を Champions 集合(calc の 359 種から内部フォーム Aegislash-Both を除く)に差し替え、`metadata.json` の `speciesScope`・`speciesCount`・`generation` を更新。
  3. Champions で変わった技(スナップトラップ等)の期待値は、gen 0 の値が既に反映するため、`known_diffs.yaml`(人間の承認が必要)を増やさずに済む見込み。
- 上げる前に、次を確認する: 0.10.0 → 0.12.0 で gen9 の出力が変わらないか(未確認)、`calculateChampions`(1194 行、独立した実装)とエンジンの丸め・補正順(ADR-0004/0008)が一致するか(未確認)。
  期待値が動く場合は、理由をコミットメッセージに書く(絶対ルール6)。

### 6. ヌケニンの扱い

- 使用可能集合に無いので、**ゴールデンでは除外のまま**とする(HP=1 特例の議論は、追加されたときに再検討する)。
- engine の HP 式(種族値 + 75 + SP)は変えない。ヌケニンが使用可能になった場合は、calc の実装(種族値 1 なら HP=1 固定)に合わせるかを、そのとき ADR で決める。

### 7. 持ち物の使用可否とエンジンの検証を分ける

- エンジンのゴールデンは、持ち物の**効果の計算式**を検証する。使用可否とは独立に、こだわり系・とつげきチョッキ等のアダプタを維持する。
- マスタの `available` は、pokedex-svc の一覧・逆算の候補抽出・UI の選択肢にだけ効かせる。

## 根拠

- **A を一次にする理由**: (1) 既に `tools/golden` の期待値生成器として採用済みで、依存・ライセンス(MIT)が増えない。
  (2) 使用可能集合と SP 式が同じ版・同じデータに入っており、**ゴールデンとマスタが同じ出所になる**(集合と期待値の不一致が起きない)。
  (3) npm の版を pin でき、再現性がある。(4) 集合の項目単位が B・C と整合した(持ち物 166 件が A と B で完全一致、種族は C の 231 種と一致)。
- **B を照合・補助に使う理由**: 習得技とメガ石の対応は A に無い。また B は独立した別の実装で、A の取りこぼしを検出できる。
- **C を日本語名に使う理由**: 機械可読の日本語名が他に無い。PokeAPI 自体は安定しているが、Champions 固有の値(技の威力)は反映されていないので、名前と図鑑番号・フォーム一覧に限定する。
- **スナップショット方式の理由**: 公式の API は無い(確認できた範囲)。外部に定期的に取りに行くと、(a) 上流の変更で突然壊れる、(b) レート制限・規約の問題(PokeAPI は「ローカルにキャッシュせよ」と明記)、
  (c) CronJob が失敗する要因が増える、(d) ゴールデンとの再現性が失われる。コミット済みなら、レビュー可能な差分として更新を確認できる。
- **コミット済みの補正定義が必要な理由**: 外部ソースはどれも 4096 基準の補正値を持たない(§8)。

## 却下案

- **攻略サイトのスクレイピング(Serebii・GameWith・Game8・やっくん等)**: 規約が未確認で、やっくんは自動取得を 403 で拒否した。
  一覧の一次情報でもない(ゲーム内が一次)。採用しない。
- **Bulbapedia の表を取り込む**: CC BY-NC-SA 2.5 で、継承条件があり、robots.txt に `Crawl-delay: 5` もある。自分用でも取り込むデータ形式のライセンスが複雑になる。人間の確認なしでは採用しない。
- **コミュニティ製 GitHub データセット(D)を一次にする**: 出所が A/B の再配布で、ライセンス表記も NOASSERTION。片方は更新が止まっている(2026-04-16)。照合の参考にとどめる。
- **Showdown だけを一次にする**: TS の継承構造(`inherit: true`)を実行して集合を得る必要があり、動かさずに読むと `isNonstandard` の解釈を誤る(gen9 の値を継承する項目が多い)。A のほうが集合が明示的。B は照合用とする。
- **実行時に PokeAPI / Showdown から毎回取得**: 上記の再現性・規約・障害の理由で却下。
- **ゲームデータの解析**: 適法性が不明で、調査もしていない。

## 限界

- **使用可能集合の「正」は未確定**。A と B が食い違う項目(技 12 件、種族 37 件)がある。集合が動く(レギュレーション更新)ため、常に古くなる。
- WebFetch 経由の情報(Bulbapedia・Serebii・GameWith・公式ニュース)は要約を通しており、数値は原文で再確認していない。
  **決定の根拠は主に [ローカル検証]**(A・B・C の実データ)に置いた。
- 種族値・特性が gen9 から変わっていないという結論は、A・B が gen9 のデータを継承しているという事実と、変更の実例が見つからなかったことに基づく。
  **ゲーム内での実測ではない**。
- A の `calculateChampions` の計算の正確さ(実機との一致)は検証していない。ゴールデンの oracle を A に切り替えると、A の誤りをエンジンが「正しい」と学習するリスクがある(この点は元の gen9 でも同じ)。
- メガ石とメガフォームの対応、習得技の網羅性は、A に無く B に依存する(B の習得技ファイルは約 348 KB。内容は精査していない)。
- 公式の使用可能一覧を Web で機械可読に得る手段は、確認できていない(存在しないとは断定できない)。

## 影響

### P2-2(スキーマと importer)

- テーブルの追加・変更点(requirements の骨格に追加):
  - `species`: `key`(`{図鑑番号4桁}-{フォルム3桁}`)、`dex_no`、`form`、`name_ja` / `name_en`、タイプ、種族値、`is_mega`、`base_species_key`、`required_item_id`(メガ石)、`available`
  - `moves`: 威力・タイプ・分類・命中・PP・優先度(Champions の値)、`available`
  - `items` / `abilities`: 名前、`available`。**補正定義は別ファイル(人が管理する YAML 等)から取り込む**(ItemEffect/AbilityEffect。§8)
  - `learnsets`: B の習得技から(A に無い)
  - `data_versions`: 各ソースの版(calc 0.12.0、Showdown commit、PokeAPI 取得日)、`regulation`、チェックサム、取込日時
- **importer の入力はコミット済みのスナップショット**。ネットワークを使わない。冪等。トランザクションで置き換える。
- **フォルム番号の採番表**(species key → フォルム番号)をリポジトリで管理する(外部ソースに無い)。既存の番号は変えない(追加のみ)。
- 検証(importer または生成ツールでテスト化):
  - A と B の集合差の報告(未裁定の差があれば失敗)
  - A の基底種数と PokeAPI の Champions 図鑑(231 種)の照合
  - 補正定義の網羅性(使用可能な持ち物のうち、補正を持つと判定されるものに定義がある/「補正なし」と明示されている)
  - 日本語名の欠落件数の報告
- 見た目だけ違うフォーム(Vivillon の模様違い等)は Showdown にだけあるので、取り込まない方針を推奨(要確認。全 35 件の内訳は未確認)。

### P2-3(pokedex-svc)

- 一覧・検索は `available` の項目に限る(オプションで全件)。日本語名の前方一致の欠落(§5)は、英語名へフォールバックする。

### ゴールデン・テスト戦略

- §決定 5 のとおり、集合・oracle の差し替えは P2 内の別タスク。`docs/test-strategy.md`(ゴールデンの節)の「@smogon/calc は 0.10.0 に固定」は、そのときに更新する。
- plan.md の P2-1 の子項目(ヌケニン)は、「使用可能集合に無いので除外のまま」と読み替えられる。

### 他の文書

- docs/requirements.md の「逆算の持ち物候補」の例、ADR-0005 の例は、M-C の実際の持ち物集合(§4)と食い違う。**書き換えは人間の確認後**(本タスクでは変更しない)。
- CLAUDE.md の前提(Lv50・個体値31・SP 32/66・HP/他の式・`8×SP−4`)は、**調べた範囲で食い違いは無かった**。
  追加で書くべき事実: 使用可能集合がレギュレーション制であること、メガが別の種族であること(CLAUDE.md には記述なし)。

## 人間の確認事項

1. **使用可能集合の「正」の決め方**: A(@smogon/calc 0.12.0)を一次、B(Showdown)を照合とする案でよいか。ゲーム内の Roster Info・持ち物・技との定期照合は誰が、いつ行うか。
2. **規約の解釈**: A・B は MIT だが、データの元は任天堂・ゲームフリークの知的財産。リポジトリは現在リモートなし(自分用)。
   スナップショットをコミットすることの可否、将来リポジトリを公開する場合の扱い。PokeAPI のデータは利用条件が README に明記されていない(コードは BSD-3-Clause)。
   Bulbapedia は CC BY-NC-SA 2.5。Serebii の利用規約は未確認。**適法性は断定していない**。
3. **対象レギュレーション**: 現行(M-C 相当)のみ持つ案でよいか。過去の集合が要るか。calc の Champions 世代と Showdown の `champions`(M-C)が同じ集合を指すことの確認。
4. **食い違う技の裁定**(§3): Anchor Shot 等 11 技(A のみ)、Pound(B のみ)、Snap Trap のタイプ(A/B は Steel、Serebii の要約は逆向きに読める)、Growth のタイプ(B と Serebii は Grass、A は Normal のまま)。ゲーム内で確認して補完ファイルに書く必要がある。
5. **持ち物の例の見直し**(§4): こだわりハチマキ・こだわりメガネ・とつげきチョッキ・しんかのきせきが現行で使用不可の可能性が高い。requirements の逆算の持ち物候補と ADR-0005 の例を直すか。
   直す場合、「攻撃側: こだわり系」の分類の扱い(空でよいか)も決める。
6. **ヌケニン**: 現行の集合に無い前提でよいか。追加されたときに HP=1 特例を残すか(calc は残す)。
7. **ゴールデンの oracle**: @smogon/calc を 0.12.0 に上げ、Champions 世代で期待値を生成する案でよいか。0.10.0 → 0.12.0 の gen9 出力の変化と、`calculateChampions` の独立実装の信頼性を、誰が確認するか。
8. **日本語名の欠落の補完**: 新規の特性・メガ石・メガフォーム名を人手で補完する運用でよいか(欠落は英語名でフォールバック)。
9. **更新運用**: スナップショットを手動更新して CronJob は冪等な照合だけにする案でよいか(学習目的で CronJob を残す理由は残る)。
10. **見た目違いフォーム**(Vivillon の模様違い等)と、メガ石 ↔ メガフォームの対応をマスタにどう持つか(エンジンの入力は「メガ後の種族を直接選ぶ」形でよいか)。
