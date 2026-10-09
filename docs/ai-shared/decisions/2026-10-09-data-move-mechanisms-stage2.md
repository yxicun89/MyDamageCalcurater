## 2026-10-09: 技の機構の段階2(I-data-6。データレーンから Web・iOS・API へ)
Decision: ADR-0143 を実装した(受け入れ条件と失敗するテストを先に置き、engine・共通マスタ・importer・DB・pokedex-svc・calc-svc・WASM・Web・ゴールデンまで)。既存の入力と種族の重さだけで決まる技の処理(威力の式・状態/持ち物/天候/フィールドの条件・タイプ・相性・優先度・壁)を engine の閉じた語彙 `MoveRule` にし、技 → 語彙は `effects.json` の新しい節 `moveRules`(手書きのデータ)で持つ。使用可能な攻撃技で印が残る 67 技のうち 36 技が対象、31 技は段階3(残り HP・対戦の履歴・フォルム等の入力が要る)。
Reason: 段階1(ADR-0142)の後も、対戦でよく使う重さ・素早さ比・状態・天候・フィールド依存の技が「未対応」の印付きで誤った数値のままだったため(issue #271 / #270)。
Impact:
- 内部 API(`GET /internal/pokedex/master`): 省略可の `MasterMove.rule`(不透明なオブジェクト)と `MasterSpecies.weightHg`(hg の整数)が増える。calc-svc はキーが無いことを「定義なし / 重さ不明」と読む。
- 公開 API: 省略可の `Move.mechanismParams`・`Move.rule`・`SpeciesDetail.weightHg` が増える。**Web レーンへの依頼**: 表示の変更は不要(印が減るだけ)。`onlineSource.ts` の写しはこの PR で入れる(テスト `onlineSource.moveRules.test.ts`)。IndexedDB の古いキャッシュは新しいキーを持たないので、オフラインでは従来どおり印が残る(キャッシュの版は上げない)。**iOS レーンへの依頼**: 計算は HTTP(calc-svc)なので変更不要。生成型の再生成だけ(省略可のキーなので既存のコードは壊れない)。
- WASM(Web 内部): 技の `rule`(camelCase。PascalCase も受ける)、種族の `weightHg`、持ち物の `isMegaStone`、特性の効果の `weightMod`。
- DB: migration 000015(`move_rules`)・000016(`species.weight_hg`)。マージ前に main の最新の版を確かめ、先に別の版が入っていたら繰り下げる。
- 多段の回数を利用者が選ぶ入力・現在 HP の入力は段階3(API・Web・iOS レーンで別 ADR)。
- デプロイ順: migrate → アプリ(pokedex-svc・calc-svc・Web の WASM)→ `make import-fetch`(重さの取り直し)→ 取り込み → master-release。先に取り込むと新しい節・効果のキーを古い importer / calc-svc が拒否する。
