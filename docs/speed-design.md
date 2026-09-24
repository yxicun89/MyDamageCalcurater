# 素早さ比較 設計書(素早さレーン)

- 更新日: 2026-09-24
- 状態: SP0〜SP5 すべて完了(main 統合済み。実データでの疎通も確認済み)。設計の正はこの文書と `docs/adr/0600〜`(素早さレーンの帯)
- ユーザーの仕様: `docs/plan.md` の「SP: 素早さ比較」と `docs/ai-shared/DECISIONS.md`(2026-09-22)

## 1. 目的

素早さ比較サイトを見比べて自分の数値を算出する手間を減らす。**左に使用可能なポケモン全員の素早さの表**(速い順)、
**右に自分のポケモン**を置き、自分の素早さの実数値が表のどこに入るかを一目で分かるようにする。

## 2. 位置づけ

- ダメージ計算(damage-calc)・タイプバランス(balance)と並ぶ 3 つ目の独立サービス `services/speed/`(ADR-0012 のサービス境界に従う)。
  他サービスの実行時 API に依存しない。
- 実数値の式(Lv50・個体値31固定・性格補正・ランク補正)は **engine の公開 API**(`engine.RealStats` / `engine.EffectiveStat`)を呼ぶだけで、自前で持たない。
  engine に無い「こだわりスカーフの素早さ ×1.5」だけを speed のコアが持つ(ADR-0600 §3)。
- 画面は Web の独立したタブ(`web/src/speed/`。SP3)。iOS は後で。

## 3. 構成

```text
services/speed/
├─ api/openapi.yaml        # speed 自身の API 契約(ルートの api/openapi.yaml とは別。balance と同じ)
├─ cmd/api/                # 起動・設定の読み込み(環境変数)
├─ cmd/checkreadmodel/     # デプロイ前に read model をサービスと同じ loader で検証するツール(ADR-0603)
├─ internal/api/           # oapi-codegen の生成物(手で編集しない。make speed-gen)
├─ internal/speed/         # 純粋な Go のコア(I/O なし。engine を呼ぶ)
├─ internal/httpapi/       # Echo の HTTP アダプタ(検証・エラー変換)
├─ internal/master/        # read model の読み込み(SP0〜SP3 は架空データ、SP4 以降は pokedex export の実データ。同じ loader)
├─ testdata/               # 架空データの example
├─ scripts/                # smoke・k3d への read model デプロイ・GitOps の検査・イメージの push
└─ deploy/
   ├─ k8s/base                    # Deployment・Service・Ingress /api/speed
   ├─ k8s/overlays/local          # 架空データの read model(ConfigMap)
   ├─ k8s/overlays/local-readmodel # pokedex export の実データ(ADR-0603)
   ├─ k8s/overlays/gitops         # digest 固定(ADR-0605)
   └─ argocd/                     # Argo CD Application pokecalc-speed(ADR-0605)
```

Ingress は `/api/speed`(balance の `/api/balance` と同じ形)。

## 4. 素早さの計算(SP0 のコア)

入力は「種族の素早さ種族値・素早さ SP(0〜32)・性格の補正(下降 / なし / 上昇)・ランク(-6〜+6)・こだわりスカーフの有無」。

1. 実数値 = `engine.RealStats`(= floor((種族値 + 20 + SP) × 性格補正)。性格補正は ×11/10 / ×9/10 の整数演算)
2. ランク補正 = `engine.EffectiveStat`(+n: ×(2+n)/2、-n: ×2/(2+n)、floor)
3. こだわりスカーフ = 4096 基準の補正 6144(×1.5)を五捨五超入で掛ける(ランクの後。Showdown の `getStat` → `ModifySpe` の順)

詳細・根拠は ADR-0600 §3。

## 5. 表の行(ユーザーの仕様。SP1)

使用可能な各ポケモンについて 6 行:

| 行 | SP | 性格 | ランク | スカーフ |
|---|---|---|---|---|
| 無振り | 0 | 補正なし | 0 | なし |
| 準速 | 32 | 補正なし | 0 | なし |
| 最速 | 32 | 上昇 | 0 | なし |
| 最速スカーフ | 32 | 上昇 | 0 | あり |
| 最速+1(ニトロチャージ等) | 32 | 上昇 | +1 | なし |
| 最速+2(こうそくいどう等) | 32 | 上昇 | +2 | なし |

- 表は最初から速い順。**同じ実数値はまとめて同速として表示**し、同速の中は図鑑番号順(pokemonId の昇順)、同じポケモンなら上の表の行の順(2026-09-22 ユーザー回答)。
- 道具(スカーフ)・ランクで行を絞り込める。
- 表に載せる範囲は**既定のレギュレーションの使用可能集合**(2026-09-22 ユーザー回答)。レギュレーションはデータで切り替え、ロジックに直書きしない。

## 6. 自分のポケモン(SP2)

- 最小の選択: ポケモン → 「無振り / 準速 / 最速」→ こだわりスカーフ on/off。
- オプション(2026-09-22 ユーザー回答): 素早さ SP 0〜32・性格の補正 3 通り・ランク -6〜+6・スカーフ on/off を自由に選ぶ。または**実数値を直接入力**して位置だけを見る。
- 結果: 自分の実数値と、表の中の位置(自分より速い行・同速の行・遅い行の境目)。同速なら「同速」と明示する。

## 7. データ(read model)

- SP0〜SP3 は speed 内の暫定の read model(架空データの JSON。`SPEED_POKEMON_PATH`)を使っていた。形式は ADR-0600 §4。
- SP4 でデータレーンの P2-3 の read model(`pokedex export` の `speed-pokemon.json`)に切り替えた(ADR-0603)。同じ形式・同じ
  loader(`master.LoadPokemonFile`)で読むだけで、adapter の差し替えは無い。2026-09-24 に実データ(348 pokemon)で疎通確認済み
  (`docs/runbooks/speed.md` 節3)。
- 実マスタ・公式画像はコミットしない(ADR-0002)。画像が無くてもタイプ色のエンブレムで成立させる(Web 側)。

## 8. 段階(2026-09-24 時点で SP0〜SP5 すべて完了)

| 段階 | 内容 | ADR |
|---|---|---|
| SP0 | この文書・services/speed の基盤(コアの素早さ計算・read model・`/healthz`・ポケモン一覧 API・Kustomize) | ADR-0600 |
| SP1 | 表(6 行の生成・速い順・同速のまとめ・絞り込みの API) | ADR-0601 |
| SP2 | 自分のポケモンの位置(最小の選択 + オプション → 実数値 → 表の中の位置) | ADR-0602 |
| SP3 | Web の素早さ画面(`web/src/speed/`。左右の配置・自分の位置の強調・表の絞り込み UI) | ADR-0604 |
| SP4 | pokedex の read model への切り替え、k3d の疎通(実データで確認済み) | ADR-0603 |
| SP5 | GitOps(digest 固定の overlay と Argo CD Application `pokecalc-speed`。balance のクラスタ内レジストリ・Argo CD を共有) | ADR-0605 |

実際にクラスタへ Argo CD の Application を適用する操作(`docs/runbooks/speed.md` 節5〜10)は、共有クラスタへの変更のため
人間の確認のもとで必要になったときに行う(まだ実施していない)。
