# ADR-0303: タイプバランスの画面(P4-12)— balance API をそのまま使う・例データの ID を揃える

- 状態: 採用(Web レーン、2026-09-22。ユーザー要望「タイプバランスの画面を作る」。担当は Web レーン(DECISIONS.md 2026-09-22))
- 関連: plan.md P4-12、docs/type-balance-design.md §6・§10、`services/balance/api/openapi.yaml`(契約の正)、ADR-0014・0016・0017・0400(balance)、
  ADR-0300 §1(URL で画面を切り替える)、ADR-0301 §3・§5(端末 ID と例データの書き出し)

## 決定

1. **契約**: `services/balance/api/openapi.yaml` を正とし、`openapi-typescript` で `web/src/api/balance.gen.ts` を生成する(`make gen-ts` に1行足す。手で型を書かない)。
   balance の API は変えない(タイプバランスレーンの範囲)。計算はすべて balance-svc が行い、Web は倍率・集計を計算し直さない。
2. **画面** `/balance`(ルート表に1件): 最大6体のメンバー(ポケモン・特性・技4つまで)を選ぶと、次を出す。
   - P4-12a: 防御相性(`analyze`。18タイプ × メンバーの倍率と、攻撃タイプごとのチーム集計)と攻撃範囲(`coverage`)
   - P4-12b: 仮想敵(`threats`)とおすすめタイプ(`recommendations`)
   倍率は色だけで表さず、`×4 弱点`・`×1/2 耐性`・`×0 無効` のように文字でも出す(design.md のタイプ色は補助)。
3. **ポケモン・技・特性の一覧**: balance には一覧の API が無いので、Web の `MasterData`(種族・覚える技・特性)から選ぶ。送るのは ID だけ
   (`pokemonId` = 種族キー、`moveId`、`abilityId`)。
4. **例データの ID を揃える**: いまは balance も Web も別々の架空データで、同じ `9001-000` が別のポケモンを指す。pokedex の read model に揃うまでの間、
   Web の例データを balance の read model の形(`services/balance/schema/`)にも書き出す(`web/scripts/export-example-master.mjs` を拡張。出力は `data/generated/`)。
   ローカルと E2E ではその出力で balance-svc を起動する。特性は、engine の効果定義から balance の正規化された効果(無効・吸収・倍率)に写せるものだけを書き出す。
5. **入口**: ブラウザは同じオリジンの `/api/balance/...` を呼ぶ(k3d では Traefik の Ingress が balance に振り分ける。ADR-0012)。
   開発サーバーでは `/api/balance` を `BALANCE_PROXY_TARGET` に転送する(`/api` の `API_PROXY_TARGET` と別。`VITE_` 接頭辞なし)。
   端末 ID とセッション ID は計算と同じもの(`X-Device-Id` / `X-Session-Id`)を付ける。
6. **オフライン**: balance はサーバーでしか計算しない(engine.wasm に含まれない)。通信できないときはエラーを出す(計算画面のように WASM へは切り替えない)。

## P4-12b: 仮想敵(threats)とおすすめタイプ(recommendations)

- 状態: 採用(Web レーン、2026-09-22。ADR-0400(threats)・ADR-0401(recommendations)を読み、画面側の詳細を決める)

7. **同じ画面の下に2節を足す**(タブは増やさない。design.md §10「将来: 攻撃範囲タブ」は見送り、まず1画面で完結させる)。
   - **仮想敵の診断**: P4-12a のメンバー(自分のパーティ)に加えて、`仮想敵n` を最大6体(ポケモン・特性・技1〜4)で入力する
     `MemberFields` と同じ構造の入力群(見出しだけ変える。実装は共通コンポーネント化してよい)。
     自分のパーティに1体以上・仮想敵に1体以上そろったら `threats` を呼ぶ。
     表示は仮想敵ごとに1つの表(行=自分のメンバー、列=受ける倍率(incoming)・与える倍率(outgoing))。
     `safe`/`superEffective` は **応答の真偽値をそのまま文字にする**(`安全`/`注意`、`抜群`/`ふつう`)。倍率の値から閾値を
     Web 側で計算し直さない(`incoming < 1` かどうかの判定はしない)。仮想敵ごとに `safeMembers`/`superEffectiveMembers` の集計も表示する。
   - **おすすめタイプ**: 新しい入力は無く、P4-12a の自分のパーティ(species・moves・ability)をそのまま `recommendations` に渡す。
     パーティに1体以上そろったら呼ぶ(`limit` は省略。既定10件)。表示は `defenseHoles`/`offenseHoles`(タイプの一覧)、
     `candidates` の表(タイプの組・defenseCovered・offenseCovered・該当ポケモンの一覧)、`abilityOptions`(防御の穴ごとの特性で補えるポケモン)。
     並び替え・件数のフィルタは行わない(応答の順のまま)。
8. **API クライアントの拡張**: `BalanceClient` に `threats(members, threats)` と `recommendations(members, limit?)` を足す。
   実装・エラーの扱い(`balance_unavailable`・封筒の素通し・例外を投げない)は `analyze`/`coverage` と同じ。
9. **古い応答の扱い**: 4つの呼び出し(analyze・coverage・threats・recommendations)はそれぞれ独立に「入力を変えたら計算中に戻し、
   古い応答は無視する」(P4-12a の analyze と同じパターン)。1つの呼び出しの遅延が他の表示をブロックしない。

## 却下

- **Web で相性を計算する**: balance-svc の計算(特性・集計・おすすめ)と二重になる。
- **balance に一覧の API を足してもらう**: 種族の一覧は pokedex が正で、balance に持たせると重複する。pokedex の read model に揃えば一覧は Web の MasterData と一致する。
- **safe/superEffective を倍率の文字列から Web 側で再判定する**(P4-12b §7): 「1」と「3/2」のどちらが弱点か等の閾値判断を
  TS に持ち込むと、balance-svc の定義(ADR-0400 §3「incoming < 1」「outgoing ≥ 2」)と二重管理になる。応答の真偽値を使う。
- **仮想敵をタブや別画面に分ける**(P4-12b §7): 自分のパーティを再入力させないため、同じ画面・同じパーティ入力を共有する。
