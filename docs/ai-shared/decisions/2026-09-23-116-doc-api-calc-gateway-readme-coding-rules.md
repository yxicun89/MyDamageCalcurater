## 2026-09-23: DOC-api(calc・gateway の README・手順書)の coding-rules §8 からの意図的な逸脱
Decision: `services/calc/README.md`・`services/gateway/README.md` を coding-rules §8 の5節(何をするか・構成図・ディレクトリ・コマンド・関連ADR)に沿って書き直したが、
gateway の README には「環境変数」「ルーティング」の2節も残した。§8 は「これ以外は書かない」としているが、既存の文書検査テスト
(`services/gateway/deploytest/web_docs_test.go` の `TestWebUpstreamIsDocumented`、`pokedex_docs_test.go` の `TestPokedexWiringIsDocumented`)が
環境変数表の特定の行(`GATEWAY_WEB_URL`・`GATEWAY_POKEDEX_URL`)とルーティング表の「それ以外」の行の文言を検査しており、削ると絶対ルール6(テストを弱めない)に反する。
`services/speed/README.md`(既存)にも同種の「エンドポイント」「環境変数」節があり、同じ運用パターンとして許容した。
Reason: critic 指摘(coding-rules §7「規約から外れるときは理由を書く」)。
Impact: 今後 gateway の README を §8 の5節だけに削る場合は、まず上記2テストの検査方法(README の文言ではなく実装から生成する等)を変える必要がある。
