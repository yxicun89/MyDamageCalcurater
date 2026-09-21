# ADR-0201: API レーンの依存を最新の安定版へ(Echo v5・oapi-codegen の echo5-server)

- 状態: 採用(2026-09-22)
- 関連: ADR-0001(技術スタック)、ADR-0200(calc-svc)、DECISIONS.md 2026-09-21「ミドルウェア・ライブラリ・ツールは導入時点の最新の安定版にする」(ユーザー決定)

## 背景

ユーザー決定で、言語・ミドルウェア・ライブラリは最新の安定版を正確な番号で固定する(自動追従はしない)。
`services` モジュールは Echo v4(v4.15.4)と kin-openapi v0.142.0 を使っていたが、Echo は v5 系(v5.3.1)が出ている。
oapi-codegen v2.8.0(最新)は `echo5-server` の生成に対応している。

## 決定

1. `services/go.mod` の直接依存を最新にする: `github.com/labstack/echo/v5 v5.3.1`(v4 は外す)、`github.com/getkin/kin-openapi v0.149.0`、
   `github.com/oapi-codegen/runtime v1.7.0`、ツール `github.com/oapi-codegen/oapi-codegen/v2 v2.8.0`。間接依存も `go get -u ./...` で上げる。
2. `services/internal/api/cfg.yaml` の `echo-server` を `echo5-server` にし、`make gen` で生成し直す(絶対ルール1)。
3. calc-svc を Echo v5 の API に合わせる: `echo.Context` は `*echo.Context`、`HTTPErrorHandler` は `func(*Context, error)`、
   既定の 404/405 は非公開型なので `echo.HTTPStatusCoder` でステータスを読む、`Response().Committed` は `echo.UnwrapResponse` 経由。
4. **例外**: `github.com/dprotaso/go-yit` は oapi-codegen v2.8.0 が要求する版(`v0.0.0-20220510233725-9ba8df137936`)に据え置く。
   最新版は `go.yaml.in/yaml/v4` に移っており、oapi-codegen が使う `github.com/vmware-labs/yaml-jsonpath v0.3.2` とコンパイルが通らない
   (`make gen` が壊れる)。oapi-codegen 側が追従したら上げる。
5. Go のツールチェーン行(go.work・各 go.mod の go/toolchain)はデータレーンが全モジュールで揃える(API レーンは変えない)。

## 却下した案

- **Echo v4 に留める**: ユーザー決定(最新に上げてアップデートの手間を減らす)に反する。v4 の保守終了後にまとめて移る方が手間が大きい。
- **go-yit も最新にする**: 生成ツールが壊れる。

## 影響

- balance(タイプバランスレーン)は同じ決定で先に Echo v5.3.1 へ移行済み(main)。
- gateway(P3-2)は最初から Echo v5 で作る。
