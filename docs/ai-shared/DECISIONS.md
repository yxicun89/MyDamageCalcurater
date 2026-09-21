# Decisions

## 2026-09-21: ダメージ計算とタイプバランスは同一クラスタ・別サービス
Decision: 1クラスタ上で別Deployment/Service。Ingressで `/api/damage` と `/api/balance` を分離。
Reason: 独立スケール・独立ロールバックの学習目的。
Impact: balance-svc も Docker化。共通クラスタ仕様は本ファイルで共有。

## 2026-09-21: Argo CD を共通デプロイ基盤にする
Decision: GitOpsを採用、Git上のKustomize定義を正本にする。Application はサービスごとに分割。Sync は最初 manual。
Reason: 変更履歴とクラスタ状態を一致させる。2サービス同時 selfHeal のリスクを避ける。
Impact: damage/balance 両方が Argo CD 管理対象。安定後に自動 sync を検討。

## 2026-09-21: balance-svc は pokedex-svc の API を呼ぶ(DB直結・Goモジュール共有はしない)
Decision: マスタデータ取得は pokedex-svc の REST API 経由。型定義の共有モジュール化は当面しない。
Reason: DBスキーマ変更やモジュール変更で相手のビルド・デプロイが壊れることを防ぐ。
Impact: balance-svc は独自DBを持たない(TB1時点)。将来必要なら DECISIONS.md に追記して再検討。

## 2026-09-21: manifest は Kustomize に統一
Decision: plain YAML / Helm ではなく Kustomize(pokecalc と同じ)。
Reason: 個人開発の規模でHelmのテンプレート化は過剰。pokecalcと構成を揃える。
Impact: services/balance/deploy/k8s も base/overlays 構成にする。

## 2026-09-21: fix/codex-workflow-golden は保留(main へマージしない)【撤回済み: 下の「main へマージして各自 main から作業する」を参照】
Decision: Claude Code が内容を確認したが、実装の続行もマージもしない。ブランチは 6d86382 として保全し、そのまま残す。
Reason: engine のダメージ計算コア(damage.go / modifiers.go / model.go の丸め順・補正段階の訂正)と
API 契約(api/openapi.yaml の level を 50 固定)に踏み込んでおり、「ルール違反や設計変更を含まないか」を
明確に判定できない。CLAUDE.md 絶対ルール3(golden 全件一致・known_diffs の人間承認)と AGENTS.md
(engine 変更は実装者と別の reviewer による独立レビュー)の確認が済んでいない。
なお保全時点で make test / make test-golden / make lint は成功しており、内容自体が壊れているわけではない。
Impact: 元の担当(Codex)が次にこのブランチの内容へ着手する際に新規タスクとして再検討する。
その際 AGENTS.md / CLAUDE.md は main 側が先に変更されているため、マージ時に統合が必要
(ブランチ側は共通ワークフロー版の書き換え、main 側はブランチ運用ルールと ai-shared 参照の追記)。
引き継ぎ資料は作らない。ブランチ内の ADR-0007 / ADR-0008 / docs/development-workflow.md は再検討時に参照する。

## 2026-09-21: Git ブランチ運用ルールを導入する
Decision: Claude Code は `feat/claude-<phase名>`、Codex は `feat/codex-<stage名>`、単発修正は `fix/claude-...` /
`fix/codex-...`。Phase/ステージ単位で切り、Claude はその Phase の /verify 通過後、Codex はそのステージの
テスト全件通過後に main へマージしてブランチを削除する。単発修正はそのセッション内でマージまで完了させる。
1ブランチに複数 Phase/ステージ分を積み上げない。
Reason: fix/codex-workflow-golden がレートリミットで宙に浮いたため。以後、詰まったブランチは相手に引き継がず、
保留として記録するか、ルール違反がなければもう片方が完了させる。
Impact: AGENTS.md「Git ブランチ運用」に本文、CLAUDE.md に要約を追記。Codex の TB0 は
feat/codex-tb0-foundation で新規に開始する(旧名 feat/type-balance-tb0 は使わない)。

## 2026-09-21: fix/codex-workflow-golden を main へマージし、以後は各自 main から作業する
Decision: 保留を撤回し、ユーザー判断で fix/codex-workflow-golden を main へ --no-ff マージ(ブランチは削除)。
Claude Code と Codex の作業を1つのブランチで混ぜない。相手の作業を使いたいときは、相手のブランチを一度 main に
マージしてから、各自が main から自分のブランチを切って作業する(AGENTS.md「Git ブランチ運用」に追記)。
Reason: 保留のまま Claude が続きの作業を積むと Codex の未レビュー作業と混ざる。一度 main に確定させて起点をそろえる方が
安全というユーザー判断。マージ後の main で make test / make test-golden(キャッシュ無し) / make lint / make build は成功。
Impact: AGENTS.md / CLAUDE.md の衝突は、Codex の共通ワークフロー(開始時と Git 運用・エージェントの使い分け・検証と終了時)と
main 側のブランチ運用・ai-shared 参照・Type Balance 担当範囲を両方残して統合。Codex 側の「feature/<内容> 等」は
新命名(feat/claude-... / feat/codex-...)に、「引き継ぎ文書に分ける」は「docs/ai-shared に書き、別途の引き継ぎ資料は作らない」に直した。
未確認: engine の丸め順訂正(ADR-0008)と openapi.yaml の level 固定は、実装者と別の reviewer による独立レビューを
まだ受けていない。P1-6 は [~] のまま、Claude Code 側で critic レビューを通してから [x] にする。

## 2026-09-21: Codex ブランチの main 取り込みはマージコーディネーター(Claude Code)が行う
Decision: Codex の feat/codex-* を main に取り込むのは Claude Code。Codex が完了を報告し、ユーザーが取り込みを指示したときだけ実施する。
共有ファイル5点(CURRENT_STATE.md=自セクションのみ / DECISIONS.md=追記のみ / go.work=Codex は追記しない /
ルート Makefile=Codex は services/balance/Makefile を作り include の1行は取り込み時に追記 / AGENTS.md・CLAUDE.md=自セクションのみ)の
編集規約を AGENTS.md に、取り込み手順を CLAUDE.md に定めた。
Reason: 担当ディレクトリを分けても共有ファイルでコンフリクトが起きるため。原因を規約で先に潰し、
それでも起きたコンフリクトは異常のサインとして自動解決せず報告する。
Impact: Codex は go.work とルート Makefile を編集しない。取り込み時のテスト確認は docs/type-balance-test-strategy.md を基準にするが、
この文書は未作成(Codex/ユーザーによる作成待ち)。作成されるまで Claude は取り込みを実行せず報告する。

## 2026-09-21: TB0 タイプ相性データの取得元・契約を確認待ち
Decision: 未決。TB0 のタイプ相性表は、pokedex-svc の REST API または既存 MySQL 取り込みデータのエクスポートを
入力にする必要があるが、現時点では該当 API・importer・export データがリポジトリに存在しないため実装を停止する。
Reason: `api/openapi.yaml` の pokedex API は種族・技・持ち物・性格のみで、タイプ相性取得契約がない。
`services/pokedex/` は `.gitkeep` のみ、P2-2/P2-3 も未着手であり、取得形式・バージョン・生成手順を推測できない。
既存 `engine/typechart.go` の手書き表を複製することは、指定された取得経路にもマスタ正本の API 化にもならない。
Impact: `feat/codex-tb0-foundation` はブランチ作成のみで、`services/balance/` の実装には未着手。
再開には、(1) pokedex-svc に追加予定のタイプ相性 API 契約、または (2) コミット可能なエクスポートファイルの
配置先・スキーマ・データバージョン・再生成方法の指定が必要。既存 API への変更が必要なら Codex は実装せず担当側の判断を待つ。

## 2026-09-21: P1-6(golden 照合)の独立レビューが PASS。ゴールデンの種族集合は暫定
Decision: Codex 実装の golden 照合・丸め順訂正(ADR-0008)を Claude Code の critic が独立レビューし PASS。P1-6 を完了扱いにした。
Reason: 期待値を変えた6件は critic が @smogon/calc 0.10.0 を直接呼んで新期待値が正と再現。golden は変異テストで実際に不一致を検出し、
再生成した5ファイルの SHA256 がコミット済みマニフェストと一致、make gen の生成結果も openapi.gen.go と一致。
Impact: ゴールデンの「全ポケモン」は gen9 参考集合の1392種で、チャンピオンズの使用可能性は保証しない。P2-1 で使用可能マスタ確定後に
make golden-generate で種族集合を差し替えて再生成する(plan.md P2-1)。Codex による別モデルレビュー(scripts/codex-review.sh)は
Codex のレートリミットで未実施。実施済みとは扱わない。前回エントリの「独立レビュー未実施」はこれで解消。

## 2026-09-21: 一括計算(P1-7)の防御側プリセットは engine が既定カタログを持つ(ADR-0009)
Decision: 防御側の代表調整(none/hp/hb/hd/hb_boost/hd_boost)の定義は engine の純粋関数 DefenderPresetCatalog() が既定値として持ち、
呼び出し側は BulkInput.Presets で上書きできる。API の presets(enum 配列)は engine の PresetKeys に対応する。presets 省略・空配列は
技の分類による既定セット(物理: none,hp,hb_boost,hb / 特殊: none,hp,hd_boost,hd / 変化技: none,hp)。
Reason: WASM(ブラウザ単体)と calc-svc の双方が同じ定義で動く必要があり、マスタ DB 経由だと二重管理になる。プリセットは持ち物・技・ポケモンの
マスタではなく「SP の配り方の型」で、ハードコード禁止規約の対象ではないと判断した(critic 2回が妥当と確認)。
Impact: none/hp/hb/hd は @smogon/calc で外部照合済み。hb_boost / hd_boost(H振り+B(D)補正)の定義は「H32・B(D)を上げる性格補正のみ・SP 振りなし」という
仮定で、外部照合なし。人間の確認待ち(docs/plan.md ブロッカー節に確定時の更新箇所一覧)。P3-1 で api/openapi.yaml の description を先に直す宿題あり。
Codex の別モデルレビューは未実施(レートリミット)。

## 2026-09-21: 「Codex の別モデルレビュー未実施(レートリミット)」の記述を訂正
Decision: Codex の担当はタイプバランスチェッカー(services/balance)の実装で、ダメージ計算 engine のレビュー担当ではない。
scripts/codex-review.sh による外部 Codex レビューは Claude Code が任意で呼ぶ追加確認で、実施しなくても phase は完了できる(ADR-0007)。
直前2エントリ(P1-6 独立レビューが PASS / 一括計算 ADR-0009)にある「Codex の別モデルレビューは未実施(レートリミット)」は、
「任意の外部 Codex レビューは未実施」と読み替える。レートリミットが未実施の理由だったとは確認していない。
Reason: Codex を engine のレビュー担当と誤って記録していたため。未実施の理由の断定も根拠がなかった。
Impact: engine・逆算・API 契約のタスクの完了条件は critic の PASS と make test / make test-golden で、Codex レビューは含まない。
DECISIONS.md は追記のみの規約のため、既存エントリは編集せずこの訂正で上書きする。CURRENT_STATE.md と CLAUDE_LOG.md の該当行は訂正済み。

## 2026-09-21: マスタデータ方針・防御プリセット・逆算・表示%のユーザー決定(ADR-0002 確定)
Decision: (1) oracle は `@smogon/calc@0.12.0` Champions を完全固定で使用し、Showdown は照合、公式のレギュレーション情報を使用可能集合の基準、日本語名は PokeAPI+ローカル override と責務を分ける。v1 は M-C のみで、レギュレーションはデータとして持ち直書きしない。
ゴールデンは 0.12.0 Champions へ切り替えてよいが、先に旧結果と diff して確認してから更新する。
(2) 実 Pokémon マスタデータ・生成済みスナップショット・公式画像は Git にコミットしない(`data/generated/` は .gitignore)。README に明記。商用公開時の第三者 IP は別問題として ADR に残す。
(3) M-C に無い持ち物(こだわりハチマキ・こだわりメガネ・とつげきチョッキ・しんかのきせき)は候補から除外。ヌケニンは v1 で考慮不要。
(4) 防御プリセット: hb/hd = H32+B(D)32・補正なし、hb_boost/hd_boost = H32+B(D)0+上昇性格、新設 hb_full/hd_full = H32+B(D)32+上昇性格。
(5) 逆算は H32 前提で B(D) の SP を 0〜32 探索し、補正なし/上昇性格ごとに SP の範囲で候補を返す(決め打ちしない)。
(6) アプリ表示の%は小数第1位。実機観測の整数%の丸め規則は別関数・別概念にする。
(7) WASM のブラウザ実機確認は P4-5(Chrome/Safari)。engine/wasmapi と engine/cmd/wasmexpect は CLAUDE.md の構成表に追記。
Reason: ユーザーが plan.md のブロッカーと ADR-0002 の確認事項に回答した。
Impact: plan.md に P1-10(プリセット再定義)/ P1-11(表示%の分離)/ P1-12(逆算の再設計)/ P2-1b(oracle 切替)を追加。ADR-0009・0010・0011 は各タスクで改訂する。
未回答: testdata/golden を「スナップショットはコミットしない」方針の対象にするか、技の使用可否の食い違い、メガ石対応・フォーム、importer の更新運用。plan.md ブロッカー節を参照。

## 2026-09-21: コーディング規約 docs/coding-rules.md(v2)を共通規約として起草。Codex の再確認は未了
Decision: Claude Code と Codex 共通のコーディング規約を docs/coding-rules.md に置く(目的: いつでも GitHub に公開できる状態・人が読みやすいコード・ハードコードしない)。
CLAUDE.md の「最初に読むもの」と AGENTS.md から参照する。既存コードの是正は plan.md の Phase R(R-1 監査 → R-2 是正 → R-3 make check-publishable)で、挙動を変えずに行う。
Codex の1回目レビュー(scripts 外で codex exec --sandbox read-only を実行)は「要修正」。指摘(文書の優先順位、DRY の例外、検証の置き場所、float の例外(ADR-0006)、
テスト条項の緩和、言語別ルールの追加、check-publishable は実装まで手動確認、golden の扱い)を v2 に反映した。v2 への Codex の再レビューは実行中で、結果は未確認。
Reason: ユーザーが「実装のハードコードをしない・人が読みやすいコード・いつでも GitHub に公開できる状態」の規約を Claude と Codex で決めてほしいと依頼した。
Impact: Codex が v2 を承認するまで「Codex の承認済み」とは扱わない。Codex は自分の担当セクション(AGENTS.md の Codex 担当範囲)と DECISIONS.md への追記で意見を残す。
LICENSE の方針は未定(公開前にユーザーが決める)。

## 2026-09-21: ユーザー回答(第三者データの基準・メガ・技の調査・丸め方針)
Decision: (1) 第三者データは「法的に面倒ならコミットしない、公開して問題ない/セキュリティ上問題ないならコミット可」。testdata/golden は数値と英語識別子だけなのでコミットを続ける(実マスタの代替は入れない)。
(2) メガシンカは別ポケモンとして登録し、専用メガストーンを持ち物に固定(変更不可)。(3) 技の使用可否は公式以外の攻略サイト(GameWith・ポケモン徹底攻略など)も Web から調べてよい(P2-1c)。
(4) 表示%は最小ダメージ側を切り捨て・最大ダメージ側を四捨五入(小数第1位)にして「最低これくらい入る」を示す。乱数で何%で倒せるかも出力する(P1-11)。
(5) 実機観測の丸め規則はユーザーの回答を私が解釈した内容(plan.md ブロッカー節)を、訂正があればもらう。
Reason/Impact: ADR-0002 §追加の回答、plan.md の P1-11 / P2-1c とブロッカー節に反映済み。

## 2026-09-21: コーディング規約 v2 を Codex が条件付き承認(条件2点を反映)
Decision: Codex の2回目レビューは「条件付き承認」。条件は (1) 採用済み ADR の明示的な例外まで「矛盾で作業停止」と読めないようにする (2) Go の「I/O 関数は context とタイムアウト」の適用範囲を限定する。
どちらも docs/coding-rules.md に反映した。反映後の Codex の再確認は行っていない(承認済みとは記録しない)。R-0 は完了扱い。
Impact: 以後、規約違反の是正は Phase R の R-2、新しいコードは規約に従う。監査結果は docs/audit-r1.md。

## 2026-09-21: 監査(R-1)へのユーザー回答: module path のプレースホルダ・履歴書き換え・タイプ相性表のデータ化など
Decision: (1) LICENSE は現時点で置かない(法的に面倒なものは公開しない。個人利用は問題ない)。(2) Go の module path から GitHub のアカウント名を外し、公開用プレースホルダ(`example.com/pokecalc/...`)にする(R-2-8)。
Codex の `services/balance` の go.mod も同じ形にする(取り込み時にマージコーディネーターが合わせる。Codex は新しいモジュールを作るときにこのプレースホルダを使う)。
(3) Git 履歴の作者名・メールを公開用に書き換える(R-2-9。バックアップを取り、Codex の worktree と main worktree の扱いを調整してから実行)。
(4) ADR-0002 の第三者データ抜粋(技の威力変更表・持ち物名・特性名の一覧)を削除し、出典・方法・件数・結論だけにする。今後も第三者データの一覧は ADR・docs に載せない。
(5) タイプの一覧とタイプ相性表はマスタとして DB に登録し、engine には入力として渡す(ADR-0013。ハードコードは修正が手間なので変数化する)。原則: 版やレギュレーションで変わりうる表・一覧はデータ、計算の手順・式はコード。
(6) engine が日本語の表示ラベルを持ってよい(ADR-0009 §1-a)。
Reason: ユーザーが docs/audit-r1.md の6つの判断事項に回答した。
Impact: plan.md に P1-13 / R-2-8 / R-2-9 を追加。coding-rules.md(module path・LICENSE・データ化の線引き)と CLAUDE.md(タイプ相性表)を更新。Codex は docs/type-balance-design.md 等を自分の担当範囲で確認すること(この決定で変える箇所があれば DECISIONS.md に提案を書く)。

## 2026-09-21: Claude Code から Codex へのレビュー依頼を行わない(ユーザー指示)
Decision: Claude Code は Codex を(codex exec / scripts/codex-review.sh で)レビューに使わない。レビューは critic のみ。ユーザーが別ターミナルで開いている Codex のセッションは、並行作業のため止めず、Claude Code も操作しない。
Reason: ユーザーが「Codex でレビューは止めてほしい。別ターミナルで開いているものは並行作業のため開いたままにしたい」と指示した。
Impact: .claude/skills/phase の手順6と CLAUDE.md のワークフロー表を更新。コーディング規約 v2 の Codex 確認(条件付き承認)は、この指示の前に行ったもの。今後の規約の変更の確認は、Codex が自分のセッションで DECISIONS.md に書く形にする。

## 2026-09-21: 履歴の書き換えは「公開用クリーンコピー」方式で行う(ユーザー承認)
Decision: Git 履歴の作者情報・アカウント名の書き換え(R-2-9)は、今のリポジトリと worktree(Codex の pokecalc-codex-tb0、pokecalc-main)を変更せず、公開時に書き換えたコピーを別に作る方式にする。
公開用 identity は `pokecalc-dev <noreply@example.com>`(実名・実メールは書き換え対象。スクリプトに直書きしない)。公開のタイミングまで実行しない。
Reason: ユーザーが Codex を別ターミナルで並行して動かしたい。履歴の書き換えはすべてのブランチと worktree に影響するため。
Impact: plan.md の R-2-9 を「公開用クリーンコピーの作成」に変更。前提ツールは git-filter-repo(未導入。公開時に導入)。以後のコミットの作者情報は今のまま(コピー作成時にまとめて置換される)。

## 2026-09-21: 共有状態は main 専用 worktree の docs/ai-shared を正本にする
Decision: 新しい coordination branch は作らない。`main` 専用 worktree
(`~/pokecalc-main`)の `docs/ai-shared/` だけを現在状態の正本とし、feature branch 内の
同名ファイルは履歴上のスナップショットとして扱う。Claude/Codex は個別 worktree で実装する。
Reason: 既存の共有 MD とブランチ運用を維持しつつ、feature branch ごとの CURRENT_STATE 分岐と、
1 worktree のブランチ切り替えによる相互干渉を解消するため。
Impact: 共有 MD のみ main へ直接コミットしてよい。実装は従来どおり feature branch のみ。
共有 MD の同期だけを目的とする merge/cherry-pick は行わない。詳細は README_AI_SHARED.md。

## 2026-09-21: damage-calc と balance は兄弟のドメインモノリスとする
Decision: 「1機能ドメイン=1サービス、サービス内部はモノリス」とし、damage-calc と balance の
相互 API 依存を作らない。pokemon/type/move/ability/damage-formula 単位の新サービスは作らない。
Reason: Kubernetes を理由に責務を細分化せず、個人開発で管理可能な複雑さを保つため。
Impact: 既存の未実装 pokedex-svc を balance の必須ランタイム依存にはしない。既存文書の
「balance-svc は pokedex-svc API を呼ぶ」は本決定で置き換える。詳細は ADR-0012。

## 2026-09-21: TB0 のタイプ相性表は差し替え可能な temporary adapter とする
Decision: 共通マスタの恒久正本が未確定の間、現行18タイプ相性を balance 内の temporary/static adapter として
利用してよい。ただし純粋コアは provider interface に依存し、正式な共通スナップショット確定後に差し替える。
Reason: Claude 側 P2-1 の ADR-0002 は共通マスタのコミット済みスナップショットを提案しているが人間確認待ち。
一方、タイプ相性コア・HTTP・Kubernetes 基盤はその確定を待たずに検証できる。
Impact: 前エントリ「TB0 タイプ相性データの取得元・契約を確認待ち」の停止条件は解除する。
temporary データを balance 独自の恒久正本として扱わず、API や新サービスを先行追加しない。

## 2026-09-21: balance の API 契約はサービスローカル OpenAPI を正とする
Decision: balance の契約は `services/balance/api/openapi.yaml` を正とし、oapi-codegen で型を生成する。
ルート `api/openapi.yaml` は damage/gateway の契約として Codex は変更しない。
Reason: damage-calc と balance を兄弟のドメインモノリスとして分離しつつ、仕様先行と生成型の規律を維持するため。
Impact: CLAUDE.md の「API はルート OpenAPI が唯一の正」は damage/gateway の範囲に限定して読み替える。
balance の契約変更はサービス内 spec → 生成 → テストの順で行う。詳細は ADR-0012。

## 2026-09-21: 共通マスタ候補 ADR-0002 は Claude feature branch 上の提案として参照する
Decision: ADR-0012 と CURRENT_STATE が参照する ADR-0002 は `feat/claude-p1-engine` 上にあり、main へは未統合であることを明記する。
Reason: main の共有状態を正本にした時点で、ブランチ指定のない ADR-0002 参照が main 上では辿れなかったため。
Impact: 共通マスタ方式は確定扱いにしない。Claude ブランチが通常手順で main に統合された後は main の ADR-0002 を参照する。

## 2026-09-21: 公開用クリーンコピーの作成担当をClaude Codeへ固定しない
Decision: 現在のrepositoryとworktreeは変更せず、公開用に調整したクリーンコピーを別directory・別repositoryとして作る。
作成担当はClaude Code/Codexのどちらかへ固定せず、ユーザーから依頼された側が行う。元repositoryへ公開用remoteを
追加せず、作者情報・個人accountを含むpath・秘密・第三者データ等の公開前検査が完了したコピーだけをprivate remoteへ
接続する。credentialやtoken、個人accountを含むremote URLは文書へ記録しない。
Reason: 公開準備を特定AIのfeature branchだけに置くと、次に作業するAIがクリーンコピーの場所・基点・検証状態を
把握できず、古いrepositoryで作業を再開したり、未消毒の履歴をpushしたりする危険があるため。
Impact: クリーンコピーを作成・更新した担当は、自分のfeature branchだけで完了を記録してはならない。元repositoryの
main正本`docs/ai-shared/CURRENT_STATE.md`の担当欄と自分のlogへ、秘密を含まない相対path、source commit、clean copyの
branch/commit、sanitization方式、公開前検査結果、remote設定/push状態を記録する。切替時はclean copy側の
`docs/ai-shared/`も更新し、以後の開発正本を明記する。元repository側は新正本へのpointerとして残し、以後そこで実装しない。

## 2026-09-21: 開発の正本を private のクリーンコピーへ移し、Claude と Codex の協調運用を「互いを待たない」形に改める(ユーザー指示)
Decision: (1) 履歴を消毒(作者を `pokecalc-dev <noreply@example.com>` に統一、個人用の手順書を全履歴から除去、module path とホームの絶対パスを置換)したクリーンコピーを
`~/MyDamageCalcurater` に作り、private の GitHub リポジトリ(origin)へ push した(`main` / `feat/claude-p1-engine` / `feat/codex-tb0-foundation`)。以後の開発の正本は origin の main。
旧ディレクトリはアーカイブ。(2) 互いのレートリミットで作業が止まらないよう、docs/ai-shared/COORDINATION.md を新設した: 各 AI が自分のブランチの統合まで単独で完了できる
(マージコーディネーター Claude Code の廃止、Codex の go.work / Makefile 追記を許可)、止まる前に WIP を commit・push して `Next` を書く、相手を待たず既定値付きの提案で進める、
レビューは各 AI の自己完結(相手に依頼しない)、ADR 番号は統合時に後から統合する側が振り直す。
Reason: ユーザーが「お互いのレートリミットでお互いの作業が止まるのが問題。先に調整して、それぞれレートリミットに関係なく作業できる状態にしたい」と依頼した。
Impact: AGENTS.md の共有ファイル編集規約と CLAUDE.md の「Codexブランチの取り込み手順」を更新(後者は廃止)。Claude 側の type-chart ADR は、Codex の ADR-0012(domain-service-boundaries)との衝突を避けて 0013 に振り直した。
Codex は次のセッションで COORDINATION.md を確認し、異議・修正があれば DECISIONS.md に追記する。それまでは本文の内容で進めてよい(既定案で進む原則)。


## 2026-09-21: 作業ディレクトリを ~/MyDamageCalcurater の1つに整理(Codex の worktree は必要なときだけ作る)
Decision: 旧ディレクトリ(~/pokecalc・~/pokecalc-main・~/pokecalc-codex-tb0。旧リポジトリの本体と worktree)をユーザー指示で削除した。旧 ~/pokecalc の未コミット分(P1-13 の作業。新リポジトリの `0e4616a` 以降に取り込み済み)と Git 管理外の `.reviews/`(Codex による規約レビューの出力。内容は coding-rules.md v2 に反映済み)は残していない。
さらに、Codex が止まっている間は使わない ~/MyDamageCalcurater-codex(git worktree)も `git worktree remove` で片付けた。ブランチ `feat/codex-tb0-foundation`(285a46f)は origin に push 済みで、失われたものはない。
Codex が再開するときは、次で worktree を作り直す(COORDINATION.md の「ディレクトリとブランチ」と同じ):
  `git -C ~/MyDamageCalcurater fetch origin && git -C ~/MyDamageCalcurater worktree add ~/MyDamageCalcurater-codex feat/codex-tb0-foundation`
その後 `cd ~/MyDamageCalcurater-codex && codex`。最初に `git fetch origin` し、origin/main の `docs/ai-shared/CURRENT_STATE.md` と `DECISIONS.md` を読む。
Reason: ユーザーが「複数ディレクトリがあるのが混乱の元なので、使うものだけにして」と指示した。Codex は同じ作業ツリーで Claude と同時に動かせない(COORDINATION.md)ため、Codex が動くときだけ worktree を作る。
Impact: 今あるディレクトリは ~/MyDamageCalcurater だけ。CURRENT_STATE.md の Type Balance 欄(Codex の担当)の `worktree: ~/pokecalc-codex-tb0` の記述は古い。Codex が次のセッションで更新すること(Claude は編集しない)。

## 2026-09-21: レーン制に移行し、main への統合は PR 経由にする(ユーザー決定)
Decision: 担当を AI(Claude Code / Codex)ではなくレーン(ダメージ計算 / タイプバランス)に持たせる。どちらの AI もどちらのレーンを進めてよく、
一方がレートリミットで止まったら、もう一方が同じレーンのブランチの `Next` から続ける。同じレーンは同時に1セッションだけ(`CURRENT_STATE.md` の `Active`)。
作業ディレクトリはレーンごとに1つ(`~/MyDamageCalcurater` / `~/MyDamageCalcurater-tb`)。main へは PR を作ってマージする(直接 push・直接 merge をしない)。
当面はユーザーが Max プランの Claude Code で両レーンを進め、Claude の上限で止まったときに Codex が同じプロンプトで続きを進める。
Reason: 旧運用(AI ごとの担当・ブランチ・状態)では、両 AI の起動プロンプトがどちらも「M1 の続き」を指示していたため同じ作業を別の場所で進めて衝突し、
引き継ぎも feature ブランチ内の状態が相手に見えずに失敗していた。main は Argo CD の GitOps が参照するため、PR を通して入れたい(ユーザー)。
Impact: COORDINATION.md を改訂、AGENTS.md(共有状態・Git ブランチ運用・共有ファイル・「タイプバランスレーンの範囲」)、CLAUDE.md(ブランチ・統合)、
README_AI_SHARED.md、CURRENT_STATE.md(レーン欄と Active)を更新。新しいブランチは `feat/calc-*` / `feat/tb-*`。既存の2ブランチはマージまでそのまま使う。

## 2026-09-21: 逆算の Recall の新定義を承認/実機の HP 減少は整数%で表示(ユーザー回答)
Decision: (1) P1-12 の Recall の新定義(返した SP 範囲が総当たりの正解と完全一致すること+被覆・厳密性・絞り込み。基準 80%/95% の数値は据え置き)を承認。
(2) 実機(ポケモンチャンピオンズ)の相手 HP の減少は**整数%**で表示される。丸め方(切り捨て/四捨五入)は未確認。
Reason: ユーザーが確認の質問に回答した。
Impact: 逆算の観測は整数%(`Percent`)が主の入力。丸め方が未確認なので、照合は切り捨て・四捨五入・切り上げのどれでも真値を落とさない区間のまま(ADR-0010 §R)。
丸め方が確認できたら区間を狭める(1か所の差し替え)。plan.md のブロッカー「観測ダメージの入力と丸め」は「丸め方のみ未確認」に縮小。

## 2026-09-21: 「TB0 タイプ相性データの取得元・契約を確認待ち」は解決済み(タイプバランスレーン、Claude Code)
Decision: 上記エントリの停止条件は「TB0 のタイプ相性表は差し替え可能な temporary adapter とする」で解除済みであり、TB0 は
その方針で実装・独立レビュー(PASS)・統合した。上記エントリは追記のみの規約により本文を残すが、現状の判断としては読まない。
Reason: TB0 の独立レビューで、未決のまま残った旧エントリが共有状態の読み手を誤解させると指摘されたため。
Impact: なし(記録の整理のみ)。

## 2026-09-21: TB0 を main に統合(PR #3)/TB1 のポケモンタイプ取得は balance ローカルの read model(既定案。タイプバランスレーン、Claude Code)
Decision: (1) feat/codex-tb0-foundation を PR #3 で main に統合した(Argo CD 実同期のみ人間の作業待ち。plan.md)。
(2) TB1 は request を `pokemonId` のみのまま維持し、pokemonId → タイプは `PokemonTypeProvider` の後ろの temporary adapter が
環境変数 `BALANCE_POKEMON_TYPES_PATH` の JSON read model を起動時に読む。未設定なら analyze は 503 `master_unavailable`、未登録 ID は 422 `unknown_pokemon`。
Git には schema と架空データの example だけを置く。詳細は ADR-0014。
Reason: ADR-0012(実行時依存なし)と ADR-0002(実データを Git に置かない)を両立し、共通マスタの schema(P2-2)を待たずに TB1 を進めるため。
Impact: ダメージ計算レーンは変更不要。P2-2 でスナップショット schema が決まったら、タイプバランスレーンが adapter を差し替える。
異議があれば追記すること(既定案で進む原則)。

## 2026-09-21: TB1 のタイプ取得・balance のテスト対象・相性表の出どころ(ユーザー決定。TB0 の critic セッション経由で受領)
Decision: (1) TB1 のポケモンのタイプは、まず仮の adapter と架空データでテストし(ADR-0014 の read model)、後で `data/generated/` のスナップショットを読む adapter に置き換える。
request は `pokemonId` のみ(ADR-0014 §1。type-balance-design.md §16 の未決「pokemonId のみかタイプまで送るか」への回答)。
(2) ルートの `make test` に balance を含める(「テスト漏れで成果物にエラーが出るのが嫌」)。ルート Makefile は include の1行のまま、
`services/balance/Makefile` で `test: balance-test` / `lint: balance-lint` / `build: balance-build` を前提条件として追加する。
(3) balance の相性表は、ダメージ計算レーンの P1-13(ADR-0013)でデータ化されたもの(現状 main の `testdata/golden/typechart.json`、同じ schema)を使う。
Reason: ユーザーが TB0 レビュー中に回答した(Claude Code のタイプバランスレーンのセッションが受領し記録)。
Impact: (2) により、balance が失敗するとルートの `make test` / `lint` / `build` も失敗する(ダメージ計算レーンの統合条件にも balance が入る)。
(3) は TB1 の続きとして balance 側で読む adapter を作り、TemporaryTypeChart を置き換える。ダメージ計算レーンの変更は不要
(typechart.json の schema を変える場合は balance の adapter も追従が要るので DECISIONS.md に書くこと)。

## 2026-09-21: 判断が必要なときの質問ルールと深夜の自律作業(ユーザー決定)
Decision: 日中(8:00〜23:00 JST)は判断が必要になったら区切りごとにまとめて質問し、待つ間は依存しない作業を進める。
深夜(23:00〜翌 8:00 JST)は質問せず、元に戻しやすい案で進めて DECISIONS.md に「暫定(深夜の自律判断。朝に確認)」と記録し、朝の最初の報告で確認を求める。
検証済みの PR は、マージしないと作業が止まる場合に限り深夜でもマージしてよい。CLAUDE.md の「人間の確認が必要なこと」は深夜も自動で進めない。
Reason: ユーザーが「判断が必要になった場合は定期的に質問するルールにしたい。寝ている時は反応できないので深夜は質問せず作業を実施してほしい」と依頼し、
時間帯(23:00〜翌 8:00)と深夜のマージ(止まるならしてよい)に回答した。
Impact: COORDINATION.md に節を追加、CLAUDE.md の「人間の確認が必要なこと」に1行追加。Codex は AGENTS.md から COORDINATION.md を参照して同じ規則に従う。
