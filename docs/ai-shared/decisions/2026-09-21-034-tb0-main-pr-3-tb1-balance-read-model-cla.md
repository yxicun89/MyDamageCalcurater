## 2026-09-21: TB0 を main に統合(PR #3)/TB1 のポケモンタイプ取得は balance ローカルの read model(既定案。タイプバランスレーン、Claude Code)
Decision: (1) feat/codex-tb0-foundation を PR #3 で main に統合した(Argo CD 実同期のみ人間の作業待ち。plan.md)。
(2) TB1 は request を `pokemonId` のみのまま維持し、pokemonId → タイプは `PokemonTypeProvider` の後ろの temporary adapter が
環境変数 `BALANCE_POKEMON_TYPES_PATH` の JSON read model を起動時に読む。未設定なら analyze は 503 `master_unavailable`、未登録 ID は 422 `unknown_pokemon`。
Git には schema と架空データの example だけを置く。詳細は ADR-0014。
Reason: ADR-0012(実行時依存なし)と ADR-0002(実データを Git に置かない)を両立し、共通マスタの schema(P2-2)を待たずに TB1 を進めるため。
Impact: ダメージ計算レーンは変更不要。P2-2 でスナップショット schema が決まったら、タイプバランスレーンが adapter を差し替える。
異議があれば追記すること(既定案で進む原則)。
