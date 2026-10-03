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

入力は「種族の素早さ種族値・素早さ SP(0〜32)・性格の補正(下降 / なし / 上昇)・ランク(-6〜+6)・こだわりスカーフの有無・追い風・まひ」(追い風・まひは ADR-0607)。

1. 実数値 = `engine.RealStats`(= floor((種族値 + 20 + SP) × 性格補正)。性格補正は ×11/10 / ×9/10 の整数演算)
2. ランク補正 = `engine.EffectiveStat`(+n: ×(2+n)/2、-n: ×2/(2+n)、floor)
3. 追い風(8192、×2)・こだわりスカーフ(6144、×1.5)= 4096 基準の補正をこの順で連結し(1 ステップ `M = (M × mod + 2048) / 4096`)、1 回だけ五捨五超入で掛ける(ランクの後。各補正ごとには丸めない。実数値 91 + 追い風 + スカーフ = 273)
4. まひ = 連結・五捨五超入の後に整数で `floor(v × 50 / 100)`(4096 基準の補正ではない。91 + スカーフ + まひ = 68)。はやあし等の特性は対象外

詳細・根拠は ADR-0600 §3・ADR-0607 §2〜§3。

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

- 表は最初から速い順(トリックルーム時は遅い順。ADR-0607 §4)。**同じ実数値はまとめて同速として表示**し、同速の中は図鑑番号順(pokemonId の昇順)、同じポケモンなら上の表の行の順(2026-09-22 ユーザー回答)。
- 道具(スカーフ)・ランクで行を絞り込める。
- 場の状態(ADR-0607): 表全体に**追い風**(相手側。全行に ×2、スカーフ行は連結して 1 回だけ丸める)、**トリックルーム**(実数値を変えず段の順だけを遅い順にする。同速のまとめ・段の中の並びは変えない)を掛けられる。まひは表に掛けない。
- 表に載せる範囲は**既定のレギュレーションの使用可能集合**(2026-09-22 ユーザー回答)。レギュレーションはデータで切り替え、ロジックに直書きしない。

## 6. 自分のポケモン(SP2)

- 最小の選択: ポケモン → 「無振り / 準速 / 最速」→ こだわりスカーフ on/off。
- オプション(2026-09-22 ユーザー回答): 素早さ SP 0〜32・性格の補正 3 通り・ランク -6〜+6・スカーフ on/off を自由に選ぶ。または**実数値を直接入力**して位置だけを見る。
- 自分の追い風・まひ(ADR-0607。preset / custom のみ。実数値 raw には掛けない)。表の追い風(相手側)とは別の欄で、両方 on も可。
- トリックルーム中は、結果の「速い/遅い」(素早さの比較)はそのままに、「自分より先に動く(= 遅い行)/後に動く(= 速い行)」を足して表示し、境界線は遅い順の表に引く。
- 結果: 自分の実数値と、表の中の位置(自分より速い行・同速の行・遅い行の境目)。同速なら「同速」と明示する。

## 7. データ(read model)

- SP0〜SP3 は speed 内の暫定の read model(架空データの JSON。`SPEED_POKEMON_PATH`)を使っていた。形式は ADR-0600 §4。
- SP4 でデータレーンの P2-3 の read model(`pokedex export` の `speed-pokemon.json`)に切り替えた(ADR-0603)。同じ形式・同じ
  loader(`master.LoadPokemonFile`)で読むだけで、adapter の差し替えは無い。2026-09-24 に実データ(348 pokemon)で疎通確認済み
  (`docs/runbooks/speed.md` 節3)。
- 実マスタ・公式画像はコミットしない(ADR-0002)。画像が無くてもタイプ色のエンブレムで成立させる(Web 側)。

- gitops overlay は balance と同じく initContainer(pokedex export)→ emptyDir で read model を得る(issue #237・ADR-0412)。
  speed 本体の image digest は全0の placeholder のままで**未配備**(Application も未適用)。配備は人間の確認のもとで(`docs/runbooks/speed.md`)。

## 8. 段階(2026-10-02 時点で SP0〜SP6 すべて完了)

| 段階 | 内容 | ADR |
|---|---|---|
| SP0 | この文書・services/speed の基盤(コアの素早さ計算・read model・`/healthz`・ポケモン一覧 API・Kustomize) | ADR-0600 |
| SP1 | 表(6 行の生成・速い順・同速のまとめ・絞り込みの API) | ADR-0601 |
| SP2 | 自分のポケモンの位置(最小の選択 + オプション → 実数値 → 表の中の位置) | ADR-0602 |
| SP3 | Web の素早さ画面(`web/src/speed/`。左右の配置・自分の位置の強調・表の絞り込み UI) | ADR-0604 |
| SP4 | pokedex の read model への切り替え、k3d の疎通(実データで確認済み) | ADR-0603 |
| SP5 | GitOps(digest 固定の overlay と Argo CD Application `pokecalc-speed`。balance のクラスタ内レジストリ・Argo CD を共有) | ADR-0605 |
| SP6 | 追い風・まひ・トリックルーム(特性は対象外) | ADR-0607 |

実際にクラスタへ Argo CD の Application を適用する操作(`docs/runbooks/speed.md` 節5〜10)は、共有クラスタへの変更のため
人間の確認のもとで必要になったときに行う(まだ実施していない)。

## 9. テスト

素早さは engine のゴールデンテスト(`testdata/golden/`)の対象外(@smogon/calc に「素早さ比較」の出力が無い)。式そのものは engine の
`RealStats`・`EffectiveStat` が既にゴールデンで守られているので、ここでは「それを組み合わせた結果」と「API・画面」を守る。
方針の全体は [test-strategy.md](test-strategy.md)(テスト戦略の索引から本節へ来る)。

| 層 | 対象 | 置き場所 | 実行 |
|---|---|---|---|
| ユニット | 素早さの計算(スカーフ・ランク・性格の組合せ)・表の行・位置の判定。境界値は SP 0/32・ランク ±6・無効な入力の拒否 | `services/speed/internal/speed/` | `make speed-test` |
| API(httpapi) | 表・位置・ポケモン一覧・ヘッダ検証・エラー本文・メトリクス。契約は `services/speed/api/openapi.yaml` が正 | `services/speed/internal/httpapi/` | `make speed-test` |
| read model | マスタの読込・検証(`internal/master`)と、起動前確認(`cmd/checkreadmodel`)・設定(`cmd/api`) | `services/speed/internal/master/`・`cmd/` | `make speed-test` |
| Web | 画面の振る舞い(preset/カスタム/実数値の入力・範囲外の検証・エラーの日本語表示・絞り込み)と API クライアント | `web/src/speed/*.test.ts(x)` | `make web-test`(`cd web && npm test -- speed`) |
| スモーク | 起動したサービスへの疎通(ヘッダ欠落 400・表・位置・不正入力 400・未知ポケモン 422) | `services/speed/scripts/smoke.sh`・`smoke-readmodel.sh`(実データ) | `make speed-smoke`・`make speed-smoke-readmodel`(クラスタ・サービスの起動が必要) |

合格基準: `make speed-test speed-lint` が通り、Web を触ったら `make web-lint web-test` も通る(型検査・ESLint・Prettier を含む)。
GitOps の overlay を触ったら `make speed-gitops-template-check` も通す。
外部サービス(pokedex・DB)が落ちても素早さの計算テストは成功する(read model はファイルから読み、テストは架空データ
`services/speed/testdata/pokemon.example.json` を使う。実データはコミットしない。ADR-0002)。
