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
