# Claude Code と Codex の協調運用(レートリミットで互いを止めない)

- 状態: 2026-09-21 制定(ユーザー依頼)。Codex の確認は、Codex が次のセッションで `DECISIONS.md` に記録する
- 目的: **どちらかがレートリミット・上限・セッション切れで止まっても、もう一方が待たずに最後まで作業できる**状態にする

## 原則

1. **自己完結**: 各 AI は「実装 → 検証 → main へ統合 → push」を**単独で完了できる**。相手の承認・レビュー・取り込み作業を待たない。
2. **止まる前に安全な場所へ**: 未コミットの作業を残さない。区切りを付けてブランチに commit・push し、次の一手を共有状態に書く。
3. **main は常に緑**: 未検証の作業(WIP)は main に入れない。main に入れたものは `make test` / `make lint` / `make check-publishable` が通る。
4. **相談は「既定値付きの提案」**: 相手の判断が要るときは、既定の案を添えて `DECISIONS.md` に書き、**既定案で先へ進む**。相手は後から異議を `DECISIONS.md` に書く。返事を待って止まらない。
5. **相手のブランチには触れない**: 引き継ぎ・代行・上書きをしない(ただし宙に浮いた作業を「そのまま活かす」のは下記の手順で可能)。

## ディレクトリとブランチ(作業場所)

| 場所 | 用途 |
|---|---|
| `~/MyDamageCalcurater` | Claude Code の作業ディレクトリ(`feat/claude-*` `fix/claude-*` ブランチ) |
| `~/MyDamageCalcurater-codex` | Codex の作業ディレクトリ。同じリポジトリの git worktree(`feat/codex-*` `fix/codex-*` ブランチ) |
| `origin`(private GitHub) | 共有の正本。`main` が統合先。**force push しない** |

- 同じディレクトリで両方の AI を同時に動かさない(作業ツリーが混ざる)。Codex 用の worktree は次で作る:
  `git worktree add ~/MyDamageCalcurater-codex feat/codex-<stage名>`(ブランチが既にある場合)
- 旧ディレクトリ(`~/pokecalc` `~/pokecalc-main` `~/pokecalc-codex-tb0`)は**アーカイブ**。以後そこで実装しない。
- git の作者情報は、このリポジトリのローカル設定(`pokecalc-dev <noreply@example.com>`)を使う。個人の identity をコミットしない。
- リモートの URL・認証情報を文書・コミットに書かない。リポジトリを公開(public)にするのは、ユーザーの明示的な指示と `make check-publishable-full` の後だけ。

## main への統合(各 AI が自分のブランチを自分で行う)

**「マージコーディネーター(Claude Code)」は廃止する**(旧 CLAUDE.md「Codexブランチの取り込み手順」・旧 AGENTS.md の該当記述)。

統合の条件(すべて満たすとき、自分のブランチを main へ入れてよい):
1. 自分の領域のテストが通る。Claude は `make test` と、計算を変えたら `make test-golden`、WASM 境界を変えたら `make test-wasm`。
   Codex は `docs/type-balance-test-strategy.md` に沿ったテスト(未作成の間は自分の領域のテスト全件)。
2. `make lint` と `make check-publishable` が通る。WIP・未検証の作業ではない。
3. 相手の領域(Codex なら `services/balance/` 以外、Claude なら `services/balance/`)を、共有ファイルの規約が許す範囲を超えて変更していない。

手順(main をチェックアウトしない。自分のブランチ上で完結する):
```
git fetch origin
git merge origin/main            # 自分のブランチに main を取り込み、競合はここで解決する
make test && make lint && make check-publishable
git push origin HEAD:main        # main が先に進んでいて拒否されたら、もう一度 fetch → merge → 検証
```
- **競合の解決**: `CURRENT_STATE.md` は自分のセクションを残し相手のセクションは相手側を採用、`DECISIONS.md` は両方の追記を残す。
  それ以外のファイルで相手の変更と競合したら、**推測で解決しない**。統合を保留し、`DECISIONS.md` に内容と既定案を書いて、自分の作業を続ける(統合できないだけで、止まらない)。
- 統合したら `DECISIONS.md` に「何を統合したか」を1行追記し、自分の `CURRENT_STATE.md` セクションを更新する。

## 共有ファイルの編集(自分の統合に必要な範囲は自分で行う)

| ファイル | 規約 |
|---|---|
| `docs/ai-shared/CURRENT_STATE.md` | 自分のセクションだけ編集する |
| `docs/ai-shared/DECISIONS.md` | 追記のみ。既存エントリは編集しない |
| `go.work` | 自分のモジュールの `use` 行を**自分で追記してよい**(Codex は `./services/balance`) |
| ルートの `Makefile` | 自分のサービスの `include <path>/Makefile` の1行を**自分で追記してよい**(Codex は `include services/balance/Makefile`)。ターゲット名は接頭辞(`balance-`)で衝突させない |
| `AGENTS.md` / `CLAUDE.md` | 自分の担当セクションだけ。Codex の担当は `AGENTS.md` の「Codex の実装担当範囲」のみ。他の箇所の変更が要るときは `DECISIONS.md` に提案する |
| `docs/adr/` | 番号は `git fetch origin` した後の main の最新の次を取る。**統合時に番号が衝突したら、後から統合する側が自分の ADR とその参照を振り直す**(例: type-chart の ADR は 0012 → 0013 に振り直した) |

## 止まるとき(レートリミット・上限・セッション終了の前後)

止まる前(予兆があるとき、または区切りごとに)に、自分のブランチで次を行う:
1. すべて commit する。未完了・未レビューなら `WIP(<タスク>): <何が未検証か>` の形にする(main には入れない)。
2. `git push origin <自分のブランチ>`。
3. 自分の `CURRENT_STATE.md` セクションの `Status` と `Next` に、**次にやることを具体的に**書く(再開する人が読むだけで続けられる程度)。自分のログにも要点を追記する。

止まった相手がいるとき(もう一方の AI が作業する側):
- **待たない。相手のブランチに触れない**。自分の領域の作業を続け、自分のブランチを自分で統合する。
- 相手の成果が要るときは、暫定の契約(temporary adapter・仮のインターフェース)で先へ進み、`DECISIONS.md` に既定案を書く(例: TB0 の type chart の provider)。
- 相手が再開したとき、相手は自分の `Next` から続ける。相手のブランチの WIP が main に入るのは、相手が検証して統合したときだけ。

## レビュー

- **各 AI が自分の独立レビュー**(Claude Code は critic、Codex は自分のレビュー役)を使う。**相手の AI にレビューを依頼しない**(相手の上限・待ち時間で止まらないため。ユーザー指示 2026-09-21)。
- 独立レビューのスキップや未実装ターゲットの正常終了を成功と数えない(CLAUDE.md)。

## ユーザーの確認が要ること(変更なし)

Xcode の署名・実機インストール、Codex / 外部サービスのログイン、`known_diffs.yaml` への追加(ADR 付き。人間の承認)、クラスタ・DB データの削除、
リポジトリの公開(public 化)、LICENSE の決定。

## 起動の目安

```
cd ~/MyDamageCalcurater        && claude    # Claude Code(feat/claude-*)
cd ~/MyDamageCalcurater-codex  && codex     # Codex(feat/codex-*。worktree)
```
どちらも、最初に `git fetch origin` し、`docs/ai-shared/CURRENT_STATE.md` と `DECISIONS.md`(main の最新)を読む。
