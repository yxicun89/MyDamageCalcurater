# speed(素早さ比較サービス)

使用可能なポケモン全員の素早さの表と、自分のポケモンの位置を比べる独立サービス。
設計の正は [docs/speed-design.md](../../docs/speed-design.md) と [ADR-0600](../../docs/adr/0600-speed-sp0-foundation.md)。
API の契約は [api/openapi.yaml](api/openapi.yaml)(変更後は `make speed-gen`)。

## SP0 の受け入れ条件

1. **素早さの計算**: `speed.Speed` が「実数値(engine.RealStats)→ ランク(engine.EffectiveStat)→ こだわりスカーフ(×6144/4096 の五捨五超入)」の順で値を返す。
   例: 実数値 201 はスカーフで 301、200 は 300。201 にランク +2 とスカーフで 603(逆順なら 602)。
2. **入力検証**: 種族値 1〜255・SP 0〜32・ランク -6〜+6・性格の補正 `minus` / `neutral` / `plus` の外は sentinel エラー(`ErrInvalidBaseSpeed` / `ErrInvalidSP` / `ErrInvalidRank` / `ErrInvalidNature`)。
3. **read model**: `SPEED_POKEMON_PATH` の JSON を ADR-0600 §4 のとおり全項目検証し、1 つでも不正なら全体を `ErrInvalidPokemon` で拒否する。
   架空データの例 `testdata/pokemon.example.json` が読め、local overlay のコピーとバイト一致する。
4. **起動設定**: `SPEED_POKEMON_PATH` が未設定・空なら provider なしで起動する。設定されているのに読めない・不正なら起動を失敗させる。`PORT` の既定は 8080。
5. **ヘルス**: `GET /healthz` と `GET /api/speed/healthz` は read model が無くても 200 `{"status":"ok"}`。
6. **ポケモン一覧**: `GET /api/speed/v1/pokemon` は `X-Device-Id`・`X-Session-Id` の欠落・空で 400 `invalid_request`、
   read model 未設定で 503 `master_unavailable`、正常なら 200 で pokemonId の昇順。provider のエラーは 500 `internal_error` の固定文言で、内部の文言を返さない。
7. **ビルド・検査**: `make speed-test` / `make speed-lint` / `make speed-build` が通り、ルートの `make test` / `lint` / `build` に含まれる。

## SP1 の受け入れ条件(ADR-0601)

1. **プリセット**: `speed.Presets()` が 6 つの行の型を ADR-0601 §2 の順(`uninvested` / `neutral-max` / `max` / `max-scarf` / `max-plus1` / `max-plus2`)で返し、
   各行の SP・性格の補正・ランク・スカーフが §2 の表と一致する。ID は OpenAPI の `PresetId` の enum と同じ順・同じ文字列。
2. **表の組み立て**: `speed.BuildTable(roster, presets)` が各ポケモン × 指定のプリセットを `speed.Speed` で計算し、同じ値を 1 つの段にまとめる。
   段は素早さの降順、段の中は pokemonId の昇順 → §2 の順(別のポケモン・別のプリセットでも値が同じなら同じ段)。
3. **絞り込み**: 指定したプリセットの行だけを出し、指定の順は結果に影響しない。空・未知・重複は sentinel エラー
   (`ErrNoPresets` / `ErrUnknownPreset` / `ErrDuplicatePreset`)。roster の種族値が不正なら `Speed` のエラーを包んで返す。
4. **API**: `GET /api/speed/v1/table?presets=...` は ヘッダー(400)→ クエリ(未知・重複・`presets=` の空・キーの繰り返しは 400 `invalid_request`)
   → read model 未設定(503 `master_unavailable`)→ 200 の順に判定する。provider・計算のエラーは 500 `internal_error` の固定文言。
5. **レスポンス**: `presets` 省略時は 6 行すべて。レスポンスの `presets` は実際に使った行を §2 の順で返す。
   例の read model では 35 段・48 行で、種族値 81 の `9002-000` と `9005-000` の行は同じ段(同速)に並ぶ。
6. **スモーク**: `make speed-smoke` が表の 200 と同速の段(`presets=max-scarf` の 219)・未知のプリセットの 400 を確かめる。

## コマンド

```
make speed-gen      # OpenAPI からコード生成
make speed-test     # GOWORK=off で go test
make speed-lint     # gofmt と go vet
make speed-build    # go build
make speed-kustomize
make speed-docker-build
make speed-smoke
```
