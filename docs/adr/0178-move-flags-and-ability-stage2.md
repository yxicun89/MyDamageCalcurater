# ADR-0178: 技のフラグをマスタに持たせ、フラグに依存する特性(段階2)を計算に入れる

- 状態: 提案(2026-10-09。spec-writer が受け入れ条件と失敗するテストを先に置いた。実装後に「採用」へ)
- 日付: 2026-10-09
- レーン: データ(取り込み・DB・共通マスタ・効果定義)+ engine(計算)+ API 契約(省略可のキーの追加)
- 関連: ADR-0005(データ駆動の効果定義)、ADR-0101 §3(取得元の表現のまま出し、検証・変換は Go 側)、ADR-0115(黙って落とさない)、
  ADR-0117(補正値の範囲)、ADR-0118(effects.json の写し)、ADR-0120(効果データの網羅)、ADR-0121(技の機構の表の流儀)、
  ADR-0122(変換結果の版)、ADR-0123(未対応の印)、ADR-0124(適用済みの migration を書き換えない)、ADR-0128(read model)、
  ADR-0136(技の対象の取り込み・照合)、ADR-0176(特性の段階1)、ADR-0223(技の対象の配線。Web のオフラインは後続のまま)、
  ADR-0304(Web のオンラインのマスタ)

## 背景

2026-10-04 のユーザーの実使用で「⚠この結果は正確でない可能性があります(未対応: 攻撃側の特性「punkrock」)」が出た(F-04)。
段階1(ADR-0176)は engine と効果データだけで足りる特性を計算に入れた。残りのうち、**技の性質(フラグ)を条件にする特性**は、
技のフラグがどこにも無いので表せない: Showdown の取り込み(`tools/importer/fetch-showdown.mjs` は `m.flags` を出さない)・
importer の `ShowdownMove`・DB・`services/internal/master`・`engine.Move` のどれも持たない。

あわせて調べたところ、**Web のオフライン(WASM)計算は技の対象と機構を engine に渡していない**(`web/src/master/onlineSource.ts` の
`mapMove` は id・名前・タイプ・分類・威力・優先度だけを写す。公開 API の `Move` は `target` を持つが `mechanisms` を持たない)。
オフラインではダブルの全体技の補正と、多段・威力変動などの技の「未対応」の印が黙って落ちている(ADR-0223 §5 が Web レーンの追従として
残したまま)。フラグも同じ経路で届けるので、この ADR で直す。

## 調査(事実。2026-10-09。ピン留めした Showdown champions mod と @smogon/calc 0.12.0 の Champions 世代。技の ID は書かない)

- oracle(`mechanics/champions.js`)がフラグを読む箇所:
  - 威力の連鎖(`calculateBPModsChampions`): フィールド → **6144 群**(テクニシャン・メガランチャー[pulse]・がんじょうあご[bite]・
    はがねのせいしん・きれあじ[slicing]。1回だけ)→ じゅうでん → **オーラ** → **5325 群**(ちからずく[secondaries または技名
    Electro Shot]・すなのちから・アナライズ・かたいツメ[contact]・パンクロック[sound]。1回だけ)→ とうそうしん → **スキン系 4915** →
    **4915 群**(すてみ[recoil または hasCrashDamage]・てつのこぶし[punch])→ かんそうはだ(防御側)→ 持ち物。
    攻撃側の特性は1つなので、同じ群・別の群の特性が同時に効くことはない。位置が意味を持つのは、フィールド・オーラ(両側)・持ち物との
    前後だけ。よって **6144 群はオーラの前(段階1の `PowerMods` と同じ位置)、5325 群と 4915 群はオーラの後・持ち物の前**の
    2段で足りる。
  - 技のタイプ: うるおいボイス(sound の技を水に。威力補正なし。スキン系と else-if。ウェザーボール等は変えない)。
  - 無効: ぼうだん(bullet)・ぼうおん(sound。変化技の例外はダメージに関係しない)。特性による無効と同じ位置(ダメージ 0)。
  - 最終補正の連鎖(`calculateFinalModsChampions`): 壁 → スナイパー → マルチスケイル → **もふもふ・Aura Guard の接触半減
    (攻撃側がえんかくなら掛けない)/ else-if 防御側パンクロックの音半減** → ハードロック・フィルター → フレンドガード →
    **もふもふの炎 ×2** → 持ち物 → 半減きのみ。
  - かたやぶりで無視される防御側の特性の一覧に Aura Guard・Bulletproof・Fluffy・Punk Rock・Soundproof がある。
- 使用可能な攻撃技 335 のフラグ(Showdown): contact 171・secondary 123・slicing 22・bullet 18・punch 16・sound 14・recoil 12・
  bite 8・pulse 5。変化技も sound 11・pulse 1 を持つ。
- **Showdown と oracle のフラグは、使用可能な技(Showdown の isNonstandard が null)ですべて一致**(0 件の差)。差は使用不可
  (Past)の 8 技だけで、oracle の技データがフラグを欠いている(変換では moves 表に採らない技なので照合の対象外)。
  `secondary` は Showdown の `secondary`/`secondaries` の有無 と oracle の `secondaries` が一致、`recoil` は Showdown の
  `recoil`/`hasCrashDamage` と oracle の `recoil`/`hasCrashDamage` が一致。
- 例外: oracle はちからずくを**技名 Electro Shot** でも掛ける。Showdown の Electro Shot は追加効果を持たず、oracle の技データも
  `secondaries` を持たない(フラグは一致する)。Champions でちからずくを持つ種族に Electro Shot を覚えるものは無い(learnset)。
- 特性の持ち主(Champions): パンクロック・てつのこぶし・かたいツメ 等はすべて使用可能。Aura Guard は Showdown で `Future`。

## 決定

### 1. フラグの語彙(閉じた語彙。値の正は engine)

`engine.MoveFlag`(`engine/move_flag.go`)。値は昇順で
`bite`・`bullet`・`contact`・`pulse`・`punch`・`recoil`・`secondary`・`slicing`・`sound` の 9 種。

- 7 種は Showdown の `flags` の同名のキー(真のもの)。それ以外の Showdown のフラグ(protect・mirror・wind 等 30 種)は
  ダメージに効かないので取り込まない(取得物には出し、Go 側で語彙に無いものを捨てる)。
- `recoil` は Showdown の `recoil` があるか `hasCrashDamage` が真(oracle のすてみと同じ条件)。
- `secondary` は Showdown の `secondary` があるか `secondaries` が空でない(oracle のちからずくの `secondaries` と同じ)。
- 反動・追加効果を**同じ表・同じ語彙**に入れる(別の列にしない)。engine の条件(`move_flag`)が1つで済み、migration も1つで済む。
- `AllMoveFlags()`(昇順のコピー)・`MoveFlag.Known()`。`services/internal/master` は別名(`master.MoveFlag`・`AllMoveFlags`・
  `IsMoveFlag`)を持つ。migration の CHECK はこの一覧と一致させる(layout テスト)。

### 2. データ: 取得 → 変換 → 照合 → 投入

- `fetch-showdown.mjs` は技ごとに `flags`(Showdown の `flags` のうち真のキーの**昇順の配列**。取得元の名前のまま)・
  `recoil`(`[分子, 分母]` か null)・`hasCrashDamage`(真偽)を出す。`fetch-calc.mjs`(`calc-move.mjs`)も同じ形の
  `flags`・`recoil`・`hasCrashDamage` と、`secondaries`(真偽)を出す。
- importer は Showdown・calc とも `flags` を**必須**としてデコードする(キーが無い古い取得物は `ErrInvalidInput`。
  メッセージに `flags` と取り直し(`make import-fetch`)を書く。ADR-0121 の mechanism・ADR-0136 の target と同じ)。
- 変換: moves 表に採る技(変化技を含む)ごとに、Showdown の値から §1 の規則でフラグを導き、`Output.MoveFlags []MoveFlagRow{MoveID, Flag}`
  ((技 ID, フラグ) の昇順・重複なし)に入れる。フラグの無い技は行を作らない。
- 照合(calc と Showdown の両方にある技): calc から同じ規則で導いたフラグの集合が Showdown と違えば
  `move-value-mismatch`(Detail `flags`)。**攻撃技は Blocker**(ダメージに効く)、変化技は警告(ADR-0136 の target と同じ扱い)。
  これが「本番のフラグ(Showdown 由来)と oracle のフラグが一致すること」の検査になる(calc の取得物は oracle と同じ版)。
- 変換結果の版(ADR-0122)は `MoveFlags` を含む(migrate 後の最初の取り込みで、取得元の版が同じでも全行を入れ直す)。
- 照合の要約に `moveFlags: attack=<攻撃技数> withFlag=<フラグを持つ攻撃技数>` とフラグごとの技数を出す(技の ID は出さない)。

### 3. DB: `move_flags(move_id, flag)`(migration 000013)

- ADR-0121 の `move_mechanisms` と同じ形: 主キー `(move_id, flag)`・`moves` への外部キー ON DELETE CASCADE・
  `flag VARCHAR(16) ascii_bin NOT NULL` と `chk_move_flags_flag CHECK (flag IN (<§1 の 9 種>))`。down は DROP TABLE。
  **行が無い = その技はフラグなし**。既存の migration は書き換えない(ADR-0124)。行は importer が書く。
- sqlc: `ListMoveFlags`・`ListMoveFlagsByMoveIDs`・`HasMoveFlags`(表に1行でもあるか)・`DeleteMoveFlags`・`InsertMoveFlag`。
- **「まだ取り込んでいない」の判定**: 表が空なら取り込み前とみなす(実データは接触技だけで 100 を超えるので、取り込み後に空になることはない。
  importer は技とフラグを同じトランザクションで入れ直す)。pokedex-svc は表が空のとき、技の `flags` を**キーごと省く**(不明)。
  1 行でもあれば、フラグの無い技も空配列を返す(既知のフラグなし)。

### 4. 共通マスタ・契約

- `master.MoveRow` に `Flags []string` と `FlagsKnown bool`。`master.Move` が検証する(未知の値・重複・`FlagsKnown` が偽なのに
  値がある は `ErrInvalidRow`)し、昇順にして `engine.Move.Flags` / `engine.Move.FlagsKnown` に写す。
- 内部 API `MasterMove.flags`: `type: array, items: string`、**省略可**(必須にしない)。省略は「不明」(古い pokedex-svc・取り込み前)。
  enum は付けない(ADR-0121 の mechanisms と同じ理由。検証は calc-svc の `master.Move`)。calc-svc の `buildMoves` は
  キーがあれば `MoveRow{Flags, FlagsKnown: true}`、無ければ `FlagsKnown: false`。
- 公開 API `Move`: 省略可の `flags`(items は新しい `MoveFlag` enum の参照)と、省略可の `mechanisms`(文字列の配列。
  値は engine の `MoveMechanism`。enum は付けない)を足す。どちらも昇順で返す。`flags` は取り込み前(§3)ならキーを省き、
  取り込み後はフラグの無い技も空配列。`mechanisms` は常に返す(空配列可。ADR-0121 は行が無い = 通常の技で、不明の状態が無い)。
  DB に語彙に無い値(CHECK をすり抜けた値)があれば、どちらも 503 `master_unavailable`(target と同じ扱い。黙って捨てない)。
  pokedex-svc は検索・batch の技の ID でフラグ・機構を引く(`ListMoveFlagsByMoveIDs`。機構にも同じ形のクエリを足してよい)。
- read model(balance・speed の `moves.json`)には**足さない**(ADR-0136 と同じ。未知のキーを拒否する読み込みを壊さない。
  dataVersion/checksum は変換結果の版で変わるが、read model の形は不変。既存の番人 `TestMovesReadModelEntryKeysAreFixed`)。

### 5. engine

- `Move.Flags []MoveFlag` と `Move.FlagsKnown bool`。`FlagsKnown` が偽のとき `Flags` は空でなければならない。
  `CalcDamage` は未知のフラグ・`FlagsKnown` が偽なのに値がある入力を `ErrInvalidMoveFlags` で拒否する。
- `AbilityEffect` の新しい項目(ゼロ値は「その効果なし」。段階1と同じ流儀):

| 項目 | 側 | 意味 | 連鎖の位置 | 使う特性 |
|---|---|---|---|---|
| `PowerMods` の条件 `move_flag`(`ConditionalPowerMod.Flag`) | 攻撃 | 技がそのフラグを持つとき威力に Modifier | 段階1の `PowerMods` と同じ(フィールドの後・オーラの前) | strongjaw(bite)・megalauncher(pulse)・sharpness(slicing)= 6144 |
| `PostAuraPowerMods []ConditionalPowerMod` | 攻撃 | 条件つきの威力補正(語彙は `PowerMods` と同じ) | **オーラの後**・タイプ変換の補正の前 | toughclaws(contact)・punkrock(sound)・sheerforce(secondary)= 5325、ironfist(punch)・reckless(recoil)= 4915 |
| `FlagTypeConvert *FlagTypeConvert{Flag, To}` | 攻撃 | そのフラグの技を To タイプにする(威力補正なし) | 段階1の `TypeConvert` と同じ(計算の最初。type_change の機構の技は変えない) | liquidvoice(sound → water) |
| `DefImmuneFlags []MoveFlag` | 防御 | そのフラグの技を無効にする(`Nullified = immune`) | 特性による無効と同じ位置 | soundproof(sound)・bulletproof(bullet) |
| `DefFinalModsByFlag map[MoveFlag]int` | 防御 | 技がそのフラグを持つとき最終補正に掛ける(キーの昇順) | 最終補正の連鎖で急所の補正の後・抜群の軽減の前 | fluffy・auraguard({contact: 2048})・punkrock({sound: 2048}) |
| `DefFinalModsByType map[Type]int` | 防御 | 技(変換後)がそのタイプのとき最終補正に掛ける | 抜群の軽減の後・攻撃側の持ち物の前 | fluffy({fire: 8192}) |
| `NoContact bool` | 攻撃 | 自分の技を接触しない扱いにする(防御側の `contact` の最終補正を受けない) | 判定のとき | longreach |

- 防御側のぼうおん・ぼうだん・もふもふ・Aura Guard・パンクロックは `Breakable`(かたやぶりで無視)。パンクロックの攻撃側の効果は無視されない。
- 条件の語彙 `PowerCondition` に `move_flag` を足す(`AllPowerConditions` は `max_base_power`・`move_flag`・`move_type`)。
  `move_flag` は `Flag` が既知・`MaxPower` 0・`MoveType` 空。他の条件は `Flag` 空。
- 値域: 補正値はすべて `MinEffectModifier..MaxEffectModifier`。`PostAuraPowerMods[].Modifier` は 4096 不可。`FlagTypeConvert` は
  `Flag`・`To` 必須。`DefImmuneFlags` は空でない・既知・重複なし。`DefFinalModsByFlag` のキーは既知。`Individual.Validate` と
  共通マスタのデコードの両方で拒否する(タイプの存在は engine では ErrUnknownType、マスタでは表で検証)。
- **未対応の印(フラグが分からないとき)**: `Move.FlagsKnown` が偽(古いマスタ・古いキャッシュ・取り込み前)のとき、フラグに依存する
  項目は「効かない」として計算し、**その特性に `unsupported_effect` の印を付ける**(攻撃側: `PowerMods`/`PostAuraPowerMods` に
  `move_flag` の条件があるか `FlagTypeConvert` がある。防御側(かたやぶりで無視した後): `DefImmuneFlags` か `DefFinalModsByFlag` がある)。
  フラグが分かっていれば印を付けない。理由のコードは既存の `unsupported_effect`(契約の enum を増やさない)。変化技には付けない(既存の規則)。
- 効果データ(`data/importer/effects.json` が正、`testdata/golden/effects.json` が写し)の定義(実装で入れる値):

| 特性 | 定義 |
|---|---|
| punkrock | `PostAuraPowerMods:[{move_flag, sound, 5325}]`・`DefFinalModsByFlag:{sound:2048}`・`Breakable` |
| ironfist | `PostAuraPowerMods:[{move_flag, punch, 4915}]` |
| toughclaws | `PostAuraPowerMods:[{move_flag, contact, 5325}]` |
| sheerforce | `PostAuraPowerMods:[{move_flag, secondary, 5325}]` |
| reckless | `PostAuraPowerMods:[{move_flag, recoil, 4915}]` |
| strongjaw / megalauncher / sharpness | `PowerMods:[{move_flag, bite / pulse / slicing, 6144}]` |
| soundproof / bulletproof | `DefImmuneFlags:[sound] / [bullet]`・`Breakable` |
| fluffy | `DefFinalModsByFlag:{contact:2048}`・`DefFinalModsByType:{fire:8192}`・`Breakable` |
| auraguard | `DefFinalModsByFlag:{contact:2048}`・`Breakable` |
| liquidvoice | `FlagTypeConvert:{Flag: sound, To: water}` |
| longreach | `NoContact: true` |

  これらは `tools/golden/unsupported-effects.json` から外す(14 件。残りは段階3以降)。

### 6. WASM・calc-svc・Web

- WASM の技の DTO に `flags`(文字列の配列)。**キーが無い・null は不明(`FlagsKnown` 偽)、配列は既知**(空配列 = フラグなし)。
  未知の値は `invalid_enum`(パスは `<技>.flags[i]`)。特性の効果の DTO は新しいキーを camelCase で受ける
  (`postAuraPowerMods`・`flagTypeConvert`・`defImmuneFlags`・`defFinalModsByFlag`・`defFinalModsByType`・`noContact`。
  `powerMods` の要素の `flag`)。入れ子は段階1と同じく PascalCase も受ける。
- calc-svc の Store は新しい項目をディープコピーして返す(スライス・map・ポインタ)。`engine.Move.Flags` もコピーする。
- Web: `web/src/engine/types.ts` の `Move` に省略可の `target`・`mechanisms`・`flags` を足し、`onlineSource.ts` の `mapMove` が
  公開 API の `target`・`mechanisms`・`flags` をそのまま写す(応答に無いキーは作らない = WASM で不明として扱われる)。
  IndexedDB の古いキャッシュはこれらを持たないので、オフラインでは不明として計算し、印で知らせる(キャッシュの版は上げない)。

### 7. ゴールデン

- 生成器(`tools/golden/generate.mjs`)の `vector()` は `options.moveFlags` のとき、oracle の技データから §1 の規則で
  `Move.Flags`(昇順)と `Move.FlagsKnown: true` を書く(ADR-0222 の `moveTarget` と同じ。既存のベクタのバイト列は変えない)。
- 効果定義に段階2の項目があれば、apply/control の対を作り、効果の有無で oracle が変わる/変わらないを確かめる
  (`effects/<id>/flag/<flag>/...`・`effects/<id>/postaura/<flag>/...`・`effects/<id>/immune/<flag>/...`・
  `effects/<id>/final/<flag>/...`・`effects/<id>/finaltype/<type>/...`・`effects/<id>/flagconvert/<flag>/...`・
  `effects/<id>/nocontact/...`)。Breakable の定義は `.../breakable` を持つ。
- **連鎖の位置は oracle では見分けられない**: 連鎖の最初の2つの補正は掛ける順によらず同じ値になり、攻撃側の特性は1つなので、
  位置が結果に出るのは「フィールドの補正・オーラ・特性の補正」が同じ技に3つそろうときだけ。Champions ではフィールドの補正
  (でんき・くさ・エスパー・ミストのドラゴン)とオーラ(フェアリー)が同じ技に掛からない。そこで位置は engine のテスト
  (`TestFlagPowerModsChainPosition`。テスト用のでんきのオーラで、入れ替えると結果が変わる補正値を探して確かめる)で固定する。
  oracle と同じ位置にしておくのは、将来オーラ・てだすけ等の補正を足したときに黙ってずれないため。
- 2026-10-09 に、§5 の表の定義を一時的に effects.json に入れて生成器を走らせ、14 特性すべてで apply/control/breakable の
  38 件が oracle の想定どおりに変わる/変わらないこと、網羅の検査(ダメージに効くもの − 定義済み = 未対応の一覧)と
  Breakable の導出(ぼうおん・ぼうだん・もふもふ・Aura Guard・パンクロック)が通ることを確かめた(実装前の確認。元に戻してある)。
- engine の golden テスト `TestGoldenCoversStage2AbilityEffects` が、対象の特性が定義済み(印なし)で、各項目に apply/control
  (Breakable なら breakable)があり、それらのベクタが `FlagsKnown` で oracle のフラグを持つことを確かめる。

### 8. デプロイ順序

古い calc-svc・WASM は新しい効果のキーを未知のフィールドとして拒否する(段階1と同じ)。古い calc-svc は `MasterMove.flags` を
無視する(生成クライアントは未知のキーを読み捨てる)。よって:
1. migrate(000013)→ アプリ(pokedex-svc・calc-svc・Web の WASM)を入れ替える(この時点では表が空なので flags は省かれ、
   フラグ依存の特性は印付きで今までどおり)。
2. importer を新しい取得物(`make import-fetch` で取り直す)で実行し、master-release で再取り込みする
   (`effects.json` の新しい定義と `move_flags` が同時に入る)。
`docs/ai-shared/decisions/` に同じ手順を書く。

### 9. PR の分け方

1 PR にする(段階1と同じ)。データだけを先に入れても、効果データ(§5 の表)を入れない限り計算は変わらず、印も変わらない。
逆に効果データを先に入れると、フラグが届くまで印付きで計算されるだけで壊れない。どちらの順でも安全だが、
golden の対(フラグを持つベクタ)とデータの照合を同じレビューで見るため1つにまとめる。

## 結果

- フラグに依存する特性 14 件が計算に入る(oracle と全件一致)。Web のオフラインでも技の対象・機構の印・フラグが engine に届く。
- 契約は省略可のキーの追加だけ(内部 API `MasterMove.flags`・公開 API `Move.flags`・`Move.mechanisms`・`MoveFlag`)。
  iOS は生成型に追従するだけで、既存のコードは壊れない。

## 対象外(後続)

- ちからずく × Electro Shot: oracle は技名で掛けるが、Showdown・oracle とも技データに追加効果が無い(§調査)。Champions で
  ちからずくの持ち主が覚えないので、印は付けない。覚えるようになったら、技の機構(`move_specific`)か印で扱う ADR を書く。
- 段階3: HP 条件(もうか等・マルチスケイル)。かんそうはだの炎の威力補正(防御側の威力段階)。アナライズ・すなのちから・とうそうしん。
- 未対応の印の定義への Breakable(ADR-0176 の対象外のまま)。
