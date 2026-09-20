# Claude Code / Codex 開発ワークフロー

規約は `CLAUDE.md` と `AGENTS.md`、進捗は `docs/plan.md` を正とする。
この文書は手順を再利用するためのもの。各回の実行結果・未完了タスクは plan と引き継ぎ記録に残す。

## 引き継ぎ時に確認した Claude 環境

2026-09-21 時点のリポジトリ内を調査した。

| 配置 | 内容 |
|---|---|
| `CLAUDE.md` / `KICKOFF.md` | ドメイン・技術規約、M1 までの起動指示 |
| `.claude/agents/quick-scanner.md` | Haiku、関連要件・コード・ADR・テストの読み取り専用調査 |
| `.claude/agents/spec-writer.md` | Opus、受け入れ条件・失敗するテスト、API 契約先行変更 |
| `.claude/agents/implementer.md` | Sonnet、最小実装・test/golden 実行 |
| `.claude/agents/critic.md` | Opus、独立レビュー・make test、ソース変更禁止 |
| `.claude/skills/phase/SKILL.md` | タスク逐次処理、レビュー修正ループ、plan 更新、1タスク1コミット |
| `.claude/skills/improve/SKILL.md` | 改善要望を requirements/design/plan に反映し phase へ |
| `.claude/skills/verify/SKILL.md` | doctor、各テスト、up/e2e、iOS、結果と操作手順を報告。修正しない |
| `.claude/settings.json` | auto 権限、allow/ask/deny、Go 編集後の gofmt フック |
| `scripts/codex-review.sh` | Claude から任意の外部 Codex レビューを呼び `.reviews/` に出力 |
| `docs/adr/0003-kickoff-workflow-and-tooling.md` | Phase 0 は親が兼任、engine コアは親の test-first 実装 + critic 独立レビュー |

リポジトリ内に独立した `.claude/commands/`、MCP 設定、他の hooks 定義は見つからなかった。
ユーザー領域の Claude 設定・MCP・セッション履歴は今回の調査対象外で、存在しないとは判断しない。
`docs/plan.md` と Git 履歴から作業を復元する。起動プロンプトがあることは、全手順を実行した証拠ではない。

既存 phase の順序は **scanner → spec → implementer → critic → 必要時に外部 Codex**。
FAIL は implementer へ戻し、同じ失敗で最大 3 回まで。未解決は `[!]` とブロッカーに残す。
マイルストーン末尾で verify。独立タスクを並列にする規定はなく、各タスクも順次処理されていた。
ADR-0003 に記載された独立 critic レビューも、実施ログがないものは履歴だけから実施済みと断定しない。

## 役割の対応

| Claude | Codex | 変更権限・出力 |
|---|---|---|
| メイン / orchestrator | メイン | 範囲・依存・担当を決定。統合、Git 操作、plan/ADR/引き継ぎ更新 |
| quick-scanner | `scanner` | 読み取り専用。関連ファイル・仕様・テスト・リスクを短く報告 |
| spec-writer | `spec_writer` | 読み取り専用。受け入れ条件・テスト設計・API/ADR 方針を提示。必要時に設計担当を兼任 |
| implementer / engine コアの親 | `implementer` またはメイン | 指定範囲でテストを先に書き、失敗を確かめて最小実装 |
| critic | `reviewer` | 読み取り専用。仕様・差分・テストを独立確認し重大/重要/軽微を報告 |
| /verify と critic の実行検証 | `verifier` またはメイン | ソースは変更せず test/lint/build を実行。検証成果物・一時キャッシュのみ書き込み可 |
| scripts/codex-review.sh の別モデル確認 | 必要時の独立レビュー | Codex から自分を再帰起動しない。別セッションのレビューと別モデルのレビューを区別して記録 |

Codex の spec_writer はテストの設計までとし、書き込みは implementer またはメインに集める。
仕様先行・test-first の順序は維持しつつ、共有作業ツリーで仕様担当と実装担当が競合しないようにする。
Claude 側のエージェント定義・skills は維持する。軽微な作業を省略する共通例外以外の既存手順は継続できる。

## 通常タスクの順序

1. メインが Git 状態と plan・要件・ADR を読み、引き継いだ変更と作業範囲を特定する。
   main 上なら変更を保持して作業ブランチを作る。既存変更を戻してから始めない。
2. 関連コードを調べる。独立した調査がある場合だけ scanner に委譲し、メインは他の有用な調査を進める。
3. 受け入れ条件とテストを設計する。複雑な仕様なら spec_writer を使用する。
   API は OpenAPI を先に変更して生成する。設計判断は ADR に残す。
4. implementer またはメインが必要なテストを先に書く。失敗理由が期待どおりか確かめて最小実装する。
   文書や軽微な整形だけの変更に、実装をなぞるテストを追加しない。
5. 差分を固定してレビューする。engine・逆算・DB 設計・API 契約は実装者とは別の reviewer を必須とする。
   書き込みが終わった同じ差分に対する reviewer と verifier の作業は並列にできる。
   検証成果物がソースを書き換える場合は並列にせず、メインが順番を管理する。
6. 指摘を評価して修正し、影響する検証・レビューを再実行する。3 回で解決しない同じ失敗はブロッカーに残す。
   外部制約で検証できない場合も「未実施」とし、合格扱いでタスクを閉じない。
7. 最終差分・検証結果を統合し、plan と引き継ぎを更新する。コミットする場合はタスクごとに対象差分を確認する。
   マイルストーン完了時は `docs/verify-<M>.md` に実際に試せる操作・期待結果を残す。

改善要望は Claude の improve と同様、要件/デザイン → plan の `I-<番号>` → 通常手順の順で処理する。
検証だけの依頼では verify と同様、修正せず結果を報告する。
今回は重複する新しい Skills を作らず、この文書を両ツールから参照する。

## 並列化・モデル・推論の予算

- 子は通常同時最大 2、再委譲なし。単純タスクはメインだけでよい。
- 並列化するのは独立した探索・レビュー・検証。依存する仕様作成と実装や同じファイルの編集を並列にしない。
- 委譲時は目的、対象ファイル、読み書き範囲、受け入れ条件、返す結果、検証担当を明示する。
  生ログを大量に返さず、根拠のファイル・行と結果を要約する。
- モデル ID は固定せず、通常利用するモデルを継承する。scanner は low、他は medium を標準とする。
- 高推論は複雑なバグ原因・丸め順序・設計などに限り、必要な理由を作業前に説明してその作業だけで使う。
  カスタム定義の推論値が優先されるため、臨時に高推論が必要なら既定の medium 定義を使ったまま
  高推論になったと説明せず、実際に設定可能な起動方法で明示する。
- 既存 Claude 定義の Haiku/Sonnet/Opus は残す。役割を Codex の高コストモデルに一律対応させない。

## Codex 設定と権限

`.codex/agents/*.toml` に各役割を定義し、`.codex/config.toml` で子の同時実行上限を 2 にする。
現行の独立 TOML は `name`・`description`・`developer_instructions` を必須とする。
設定形式は [OpenAI 公式 Subagents](https://learn.chatgpt.com/docs/agent-configuration/subagents)
で確認した(2026-09-21)。

設定を書いたことと、このセッションでカスタム役割を実際に読み込めたことは区別する。
読込後の新しいセッションで役割を確認する。利用クライアントが役割指定を公開していなければ、
組み込みサブエージェントに同じ役割・制約を明示して委譲し、互換設定を推測で増やさない。

- scanner/spec_writer/reviewer は read-only。implementer/verifier は workspace-write だが、
  verifier は検証の生成物以外を変更しないという指示を持つ。権限は実行環境の制約に従う。
- Claude の `Bash(make *)` 等の広い allow は Codex に移植しない。
  ログイン、Xcode 署名・実機、known_diffs、人間の承認が必要な削除は `CLAUDE.md` に従う。
- `.codex/` が保護領域なら承認された書き込み手順を使う。制約を迂回しない。
- Claude の Go 編集フックは最後に `exit 0` があり失敗を隠し得る。
  Codex に複製せず変更した Go ファイルの明示的な gofmt と lint で結果を検証する。
  `make fmt` は広い範囲を書き換えるので既存変更がある場合は変更ファイルに絞る。
- 外部レビュー用スクリプトは skip/失敗でも終了 0 になり得る。出力内容を確認し、
  `.reviews/` のファイル生成だけで PASS としない。無関係な直前コミットを含む場合はレビュー範囲も明示する。

## 検証と復旧・引き継ぎ

Makefile と各 package.json の実装を確認して test/lint/build を実行する。
終了 0 でも対象テスト 0 件や未実装メッセージなら成功数に含めない。
依存取得・ネットワーク・ツール不足は理由と再実行コマンドを記録する。
未実装ターゲットとテスト失敗、環境要因を混同しない。

中断後は最初に `git status --short --branch`、差分、plan と最後の引き継ぎを読む。
実行中プロセスや生成途中ファイルを確認してから再開し、reset/clean で初期化しない。
引き継ぎには次を残す。

- 対象タスク・ブランチ・起点コミット・引き継いだ差分
- 実施内容、受け入れ条件の達成状況、設計判断と ADR
- 実行コマンド、成功/失敗/未実施、失敗の原因・試した修正
- レビュー担当・対象差分・指摘と対応、レビューの未実施理由
- 残作業・ブロッカー・次に再開する具体的なファイル/タスク
