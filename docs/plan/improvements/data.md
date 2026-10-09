# 改善要望(data レーン)

/improve で追加する。タスク ID は `I-data-<連番>`(ADR-0172)。

- [x] I-data-1 メガストーンの判定を取得元(Showdown の megaStone)からも導く・M-Mega/F-Mega をメガとして取り込む・日本語名の無いものは推測せず上書きで補う(issue #607データレーン分。ADR-0140。migration 000012。表示のフォールバックは Web・iOS レーン)
- [x] I-data-2 = F-04 段階1: 未対応の特性のうち engine と効果データだけで足りるもの(フェアリースキン系・ちからもち系・はりきり・ファーコート・テクニシャン・はがねのせいしん・フェアリーオーラ・スナイパー・カブトアーマー/シェルアーマー・かたやぶり・てんねん)を計算に入れる(ADR-0176。ゴールデン全件一致)。段階2(技のフラグに依存するパンクロック等)・段階3(HP 条件)は別 PR
- [x] I-data-3 種族の姿の日本語名を、上流 PokeAPI の form_name から「基本種名（姿の名前）」で作る(issue #607 の残り。原因は form_name の取りこぼし。ADR-0141。実データで species の fallbackIds 39 → 9 件。表示への影響なし)
- [x] I-data-4 = F-04 段階2: 技のフラグ(接触・音・パンチ・かみつき・切る・波動・弾・反動・追加効果)を取り込み(migration 000013 の move_flags・内部 API MasterMove.flags・公開 API Move.flags/mechanisms)、パンクロック・てつのこぶし・かたいツメ・ちからずく・すてみ・がんじょうあご・メガランチャー・きれあじ・ぼうおん・ぼうだん・もふもふ・Aura Guard・うるおいボイス・えんかくの 14 特性を計算に入れる(ADR-0178。ゴールデン全件一致・38 件追加。Web のオフラインで技の対象・機構・フラグが WASM に届かなかった不具合も修正)。段階3(HP 条件)は別 PR
- [x] I-data-5 技の機構の段階1: 取得元のフィールドで決まる機構(多段・固定ダメージ・一撃必殺・必ず急所・防御ランク無視・攻撃/防御に使う能力値)を engine で計算し、未対応の印を外す(issue #271 / #270 の次の段。ADR-0142。migration 000014・内部 API MasterMove.mechanismParams・WASM の hitRolls・ゴールデン mechanisms.json)。実装済み(ゴールデン mechanisms.json 232 件を含め全件一致)。利用者が回数を選ぶ入力・公開 API の1発ごとの値・一撃必殺の命中率の表示は段階2(API / Web / iOS レーン)
- [x] I-data-6 技の機構の段階2: 既存の入力と種族の重さだけで決まる技の処理(重さ・素早さ比・ランク・状態・持ち物・天候・フィールド・2タイプの相性・壁・何発目か)を engine の閉じた語彙 MoveRule で計算し、未対応の印を外す(ADR-0143。使用可能な攻撃技で印が残る 67 技 → 31 技。effects.json の moveRules・migration 000015 move_rules / 000016 species.weight_hg・内部 API MasterMove.rule / MasterSpecies.weightHg・公開 API Move.mechanismParams / Move.rule / SpeciesDetail.weightHg・ゴールデン mechanisms-stage2.json)。実装済み(ゴールデン mechanisms-stage2.json を含め全件一致。実データの dry-run の結果は ADR-0143 §結果)。残り HP・対戦の履歴・多段の回数の入力は段階3
