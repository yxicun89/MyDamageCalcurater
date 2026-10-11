## 2026-10-11: おすすめタイプの改善(偏りの除外・該当ポケモンのタイル・相性の色。ADR-0422。Web・iOS レーンへ)
Decision: (1) 補うタイプの該当ポケモンを小さなタイル(画像 + 名前)で出し、タップで構築の最初の空き枠に入れる(満杯なら押せない)。ポケモンは今の recommendations 応答の `pokemon`(read model 由来)をそのまま使う。(2) おすすめは上位 5 個(クライアントが `limit: 5`)。サーバーは「偏りのある攻撃タイプ(`weak - (resist + immune) >= 2`)に弱点(×2 以上)を持つ候補」を除外し、応答に `skewedAttackTypes` を足す(balance API 0.9.0 → 0.10.0)。(3) 相性の色: 弱点・抜群は赤のまま、耐性・いまひとつは水色系(`matchup.resist`)、無効は灰(`matchup.immune`)。緑は相性に使わない。トークンは docs/design.md に入れ、iOS は同名の `ColorToken`。
- 分担: A サーバーの除外・契約(タイプバランス)→ B Web(タイル・色・`limit: 5`)→ C iOS(同上)。`BalancePanel` は `onPickPokemon`・`canPick` を受け取り、枠への入れ方は構築側が持つ。
Reason: ユーザー要望(選びやすさ。明らかにおかしい候補を出さない。色の意味をそろえる)。
Impact: balance(サーバー・契約)、Web(BalancePanel・tokens.css・design.md)、iOS(BalanceRecommendationsView・ColorToken・生成クライアントの再生成)。
