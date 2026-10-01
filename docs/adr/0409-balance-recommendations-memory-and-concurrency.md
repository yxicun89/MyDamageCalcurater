# ADR-0409: balance recommendations の1リクエストあたりのメモリ削減と同時実行数の上限(issue #298)

- 状態: 採用(2026-10-01 実装・実測済み)
- 日付: 2026-10-01
- 関連: ADR-0401(おすすめタイプ)、ADR-0406(メトリクス)、issue #298、`services/balance/internal/httpapi/recommendations.go`、
  `services/balance/internal/balance/recommend.go`

## 背景
balance-svc は limits(CPU 100m・メモリ 64Mi)で動く。実カタログ規模(架空 ID 1,500件で再現)で
`POST /api/balance/v1/team-balance/recommendations` を同時 30 で受けると OOMKill される(issue #298。GOMEMLIMIT=48MiB でも再現)。
calc・gateway は `GOMEMLIMIT` を持つが、balance・judge・speed の Deployment には無い。同時実行数の制御も無い。

## 実測(削減前。`BenchmarkRecommendations`、1,500件・6メンバー・limit 20、Apple M5 Pro)
- 約 **3.1 MB/op**・約 **4,580 allocs/op**・約 3.6 ms/op。同時 30 で一時的に約 94 MB となり、64Mi を超える。
- `alloc_space` の内訳(memprofilerate=1): `balance.matchingPokemon` 62%、`balance.abilityOptionsFor` 23%、
  カタログの写し(`AllPokemon`)5%。つまり主因は「候補ごと(最大 20)・防御の穴ごとに、結果スライスを
  `make(..., 0, len(catalog))` でカタログ件数分あらかじめ確保している」ことで、カタログの写しは主因ではない
  (issue 本文の「写しを1回に」は効果が小さい)。

## 決定
1. **(a) アロケーション削減**: `matchingPokemon`・`abilityOptionsFor` の結果スライスの初期容量をカタログ件数にしない
   (0 から伸ばす、または候補に合う件数を先に数える)。応答は1リクエスト前と同一(順序・内容とも。既存テストで固定)。
   目標は 1 MiB/op 以下(テスト `TestRecommendationsAllocationBudget`)。実測値は実装後にここへ追記する。
2. **GOMEMLIMIT**: balance・judge・speed の Deployment に `GOMEMLIMIT` を足す。値は `limits.memory` の 75% 以上・未満
   (calc の 56MiB / 64Mi に揃える)。構造テストは `services/gateway/deploytest/go_memlimit_test.go`(calc・gateway も同じ不変条件で検査)。
3. **(b) 同時実行セマフォ**: recommendations だけを対象に、同時に計算してよい数を制限する。
   - 上限は環境変数 `BALANCE_MAX_CONCURRENT_RECOMMENDATIONS`(正の整数。未設定・空は既定 `httpapi.DefaultMaxConcurrentRecommendations = 4`。
     0・負・数値でない値は起動失敗)。`httpapi.Dependencies.MaxConcurrentRecommendations` で渡し、0 以下は既定。
   - **待たせない**。枠が無ければ即 `503` + 既存の `Error` 形式 + `Retry-After: 1`。コードは新設の `overloaded`
     (`master_unavailable` はマスタ未投入の意味なので流用しない)。ErrorCode に1値追加する API 変更なので
     `services/balance/api/openapi.yaml` を先に更新して `make gen`・`make gen-ts` 済み。
   - 枠を取る位置は「入力検証と 503 master_unavailable と ID 解決の後、`AllPokemon` の直前」。検証エラー(400/413/422)と他の
     エンドポイント・`/healthz`・`/metrics` は満杯でも従来どおり応答する。枠は `defer` で必ず返す(500 のときも)。
   - 新しいメトリクスは足さない(`http_requests_total` の status=503 で数えられる。ADR-0406)。
4. **(c) limits.memory を上げる**は採らない(原因を隠す。issue の既定案どおり)。

## 検討して採らなかったもの
- 全エンドポイントへのセマフォ: threats・move-range もカタログを走査するが、issue の再現は recommendations のみ。
  実測で問題が出たら同じ仕組みを広げる(別 issue)。
- キュー待ち(待たせる): 64Mi・100m のサービスでは待ち行列自体がメモリと遅延になる。issue の受け入れ条件は「待たせるか 503」で、
  503 の方が単純で測りやすい。

## 検証
- 単体: セマフォ超過で 503・枠の解放・既定値 4・環境変数の検証・アロケーション上限・GOMEMLIMIT の構造テスト。
- 手動(実装者): issue #298 の再現手順 2〜3(`docker run --cpus 0.1 --memory 64m`、同時 30)で `OOMKilled=false` を確認し、
  結果(成功件数・503 件数・VmHWM)をここへ追記する。

## 実装後の実測(2026-10-01)
- `BenchmarkRecommendations`(1,500件・6メンバー・limit 20、Apple M5 Pro): 削減前 約 3.1 MB/op → **約 0.53 MB/op**
  (約 4,600 allocs/op は同程度。ベンチのカタログ写し 約 15 KB を含む)。`TestRecommendationsAllocationBudget`(1 MiB/op 以下)通過。
  - `matchingPokemon` の結果スライスを `0, len(catalog)` の事前確保から 0 始まりに変更。
  - `abilityOptionsFor` が防御の穴ごとにカタログ全体を写してソートしていたため、`RecommendTypes` で1回だけソートして渡す
    (これだけで 約 1.17 MB/op → 約 0.53 MB/op。応答は同一)。
- docker(`--cpus 0.1 --memory 64m --memory-swap 64m`、`GOMEMLIMIT=56MiB`、架空1,500件の read model、同時 30 を4ラウンド):
  200 が 6〜10 件・503 `overloaded` が 20〜24 件、**`OOMKilled=false`・再起動 0**、停止後のメモリ約 9 MiB。
  `scratch` イメージのため VmHWM は取っていない。
