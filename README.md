# pokecalc スターターキット

Claude Code に渡して、M1(ブラウザで計算できる状態)まで自動で作らせるためのキットです。

## 1. 一度だけやる準備

1. Docker 実行環境を入れて起動する(Mac なら OrbStack が軽い): `brew install orbstack`
2. 必須ツール: `brew install go node jq k3d kubectl helm`、TiDB 用に tiup(公式のインストーラ)
3. 確認: `./scripts/doctor.sh`(足りないものは Claude Code も入れようとします)
4. このフォルダを git 管理にする: `git init && git add -A && git commit -m "starter kit"`

### Codex(任意・ChatGPT の課金プランで利用)
```
npm i -g @openai/codex
codex login
```
Claude Code の中で公式プラグインも入れておくと、手動レビューが楽です:
```
/plugin marketplace add openai/codex-plugin-cc
/plugin install codex@openai-codex
/codex:setup
```
ワークフローからは `scripts/codex-review.sh`(`codex exec`)で自動的に呼ばれます。

## 2. 起動

```
claude --model opus --permission-mode auto "$(cat KICKOFF.md)"
```
- 初回だけフォルダの信頼確認が出ます(1回承認すれば以後出ません)
- auto モード: 危険な操作は分類器が止め、`.claude/settings.json` の deny は常に拒否、ask は必ず確認
- 実装の大半は Sonnet(implementer)、探索は Haiku、仕様とレビューは Opus で動きます
- 中断したら `claude --continue`、または新しいセッションで `/phase M1`

## 3. 動作確認と改善

1. 完了すると `docs/verify-m1.md` に手順が書かれます
2. `make up` → ブラウザで確認
3. 直したい点は `/improve 〇〇を××にしたい`
4. 次のマイルストーンは `/phase M2`

## ファイル構成
```
CLAUDE.md            ルール・構成・ワークフロー(Claude Code 用)
AGENTS.md            Codex 用(レビューの観点)
KICKOFF.md           最初に渡すプロンプト
docs/plan.md         マイルストーンとタスク(進行状況もここ)
docs/requirements.md 要件
docs/test-strategy.md テスト戦略(全ポケモン網羅・ゴールデン・逆算の再現率)
docs/design.md       デザイントークンと画面
docs/adr/            設計判断
.claude/agents/      サブエージェント(モデル割り当て込み)
.claude/skills/      /phase /verify /improve
.claude/settings.json 権限(auto モード・allow/ask/deny)とフォーマット用フック
scripts/             doctor.sh / codex-review.sh
```
