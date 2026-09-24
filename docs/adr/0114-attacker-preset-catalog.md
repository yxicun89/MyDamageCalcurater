# ADR-0114: 攻撃側プリセットの正を engine の JSON カタログにする(issue #71)

- 状態: 採用(データレーン側。Web・iOS の追従は各レーンの後続 PR)
- 日付: 2026-09-25
- レーン: データ(ADR 帯 `0100〜`)
- 関連: issue #71、ADR-0300 §5(Web の攻撃側プリセット)、ADR-0500 §6(iOS の自分側プリセット)、
  ADR-0009(防御側プリセットのカタログ)、ADR-0010 §5.3(attacker の事前順位)、DECISIONS.md 2026-09-21・2026-09-23

## 背景

攻撃側(自分側)プリセット「無振り / A(C)特化 / A(C)振り」は、同じ規則を Web(`web/src/domain/attackerPresets.ts`)と
iOS(`ios/PokeCalcKit/Sources/PokeCalcCore/AttackerPreset.swift`)が別々に持っていた。engine には共有の定義が無い。
着手時点で両者はすでに食い違っていた。

| 項目 | Web | iOS |
|---|---|---|
| キー | `none` / `x_full` / `x` | `none` / `aFull` / `aMax` |
| 並び順 | 無振り → 特化 → 振り | 特化 → 振り → 無振り |
| SP・性格の規則 | 同じ(X = 物理・変化は atk、特殊は spa。特化は +X / −反対側) | 同じ |

Web の順序は ADR-0300 §5 の表と ADR-0010 §5.3 の attacker の事前順位(0 無振り、1 特化、2 振り)と一致する。

## 決定

1. **正は `engine/presets/attacker.json` の1ファイル**にする。engine は `//go:embed` でビルド時に読み込む
   (実行時のファイル I/O は無い。CLAUDE.md 絶対ルール2)。WASM・calc-svc も同じ定義で動く。
2. JSON はキー・順序・SP・性格の**規則**を持ち、表示の文言は持たない(文言は各クライアントの文言資源に残す)。

   ```json
   {
     "schemaVersion": 1,
     "default": "none",
     "relevantStat": { "physical": "atk", "special": "spa", "status": "atk" },
     "boostMinus": { "atk": "spa", "spa": "atk" },
     "presets": [
       { "key": "none",   "relevantSp": 0,  "nature": "neutral" },
       { "key": "x_full", "relevantSp": 32, "nature": "boost" },
       { "key": "x",      "relevantSp": 32, "nature": "neutral" }
     ]
   }
   ```

   - `presets` の並びが画面の並び順。`default` は既定の選択
   - `relevantSp` は X(技の分類から `relevantStat` で決まる関連ステータス)に振る SP。他は 0
   - `nature`: `neutral` = 無補正、`boost` = X 上昇・`boostMinus[X]` 下降
   - キーと順序は Web の現行値(ADR-0300 §5・ADR-0010 §5.3 と同じ)を採る
3. engine の API: `AttackerPresetCatalog()`・`DefaultAttackerPreset()`・`ResolveAttackerPreset(key, category)`。
   読み込みは未知フィールド・後続データを拒否し、版・キーの重複/空・SP 範囲・性格の規則・分類の網羅・`boostMinus` の整合を検証する。
   埋め込みデータが壊れていれば初期化時に panic する(黙って空のカタログにしない。`make test` で必ず検出される)。
4. Web・iOS の同期は**契約テスト**で保証する(生成はしない)。各クライアントのテストが同じ JSON を読み、自分の定義と
   キー・順序・既定・SP・性格の規則が一致することを確かめる。クライアントの実装を JSON の読み込みに置き換えるかは各レーンが決める。
5. OpenAPI・WASM 境界には今回足さない。いまの利用者(Web・iOS)はクライアント内で個体を組み立てており、
   ファイルを直接読む契約テストで足りる。サーバー側で解決する必要が出たときに API レーンが契約を足す。

## Web・iOS への依頼(後続 PR)

- Web: `web/src/domain/attackerPresets.ts` の `ATTACKER_PRESET_KEYS`・`DEFAULT_ATTACKER_PRESET`・`resolveAttackerPreset` が
  `engine/presets/attacker.json` と一致することを確かめるテストを追加する(`testdata/golden/typechart.json` と同じく、Vite の別名等で
  複製せずに読む)。現行の値は JSON と一致しているので、実装の変更は不要の見込み。
- iOS: `AttackerPreset.swift` を JSON と突き合わせるテストを追加する。キーの対応(`aFull` ↔ `x_full`、`aMax` ↔ `x`)を
  テスト内に明示するか、Swift 側の raw value を JSON のキーに揃える。**並び順が JSON(無振り → 特化 → 振り)と違う**ので、
  `allCases` の順序を揃える(画面のセグメントの並びが変わる)。性格はこれまでどおり一覧(マスタ)から規則で選ぶ。

## 結果

- 攻撃側プリセットの定義の変更は JSON の1か所で行い、各クライアントの契約テストが追従漏れを検出する。
- ADR-0300 §5 の「engine への移管」の提案(DECISIONS.md 2026-09-21)は、engine 側について本 ADR で実施した。
- 代わりに考えた案: Go のコード(ADR-0009 の `DefenderPresetCatalog()` と同じ形)。Web・iOS のテストから読めないため、
  同期の保証に WASM やコード生成が要る。JSON なら各言語のテストがそのまま読める。
