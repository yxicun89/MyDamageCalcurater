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

## 2026-09-21: マスタのフォームの持ち方・更新運用・技の使用可否の裁定の進め方(ユーザー回答)
Decision: (1) 見た目だけ違うフォーム(種族値・タイプ・特性が同じ)はマスタで1件にまとめる。性能が違うフォーム(メガ・リージョンフォーム等)は別ポケモンとして登録する(メガは既決のとおり `is_mega` / `base_species_key` / `required_item_id`)。
(2) マスタの更新は k8s の CronJob で定期取込する。取得元への負荷を抑えるため、既定の頻度は週1回、取得元ごとに版(バージョン・commit)を見て変化が無ければ取り込まない。手動の `make import` も残す。
(3) 技の使用可否(P2-1c)は既定案で進める: 複数の出典で使用不可と確認できた技は除外し、断定できない技は「使用可」として残す(計算できないより安全)。断定できなかった技の一覧(件数と ID のみ)を plan.md に載せ、後でユーザーが裁定する。
Reason: ユーザーが P2-2 / P2-1c で判断待ちになりうる点に回答した。
Impact: plan.md のブロッカー「見た目違いフォームの持ち方、importer の更新運用」を解消。P2-2 のスキーマ・importer・CronJob はこの方針で設計する。

## 2026-09-21: 人間への質問は日中だけ、深夜は質問せずに作業を続ける(ユーザー決定)
Decision: 人間の判断が必要なときは、日中(8:00〜23:00 日本時間)はタスクの区切りでまとめて既定案付きで質問する。深夜(23:00〜8:00)は質問せず、plan.md のブロッカー節に既定案付きで書いて、判断に依存しない作業を続ける(取り消しやすい判断は既定案で進めてよい。取り消しにくい操作は深夜に行わない)。朝の最初の区切りでまとめて質問する。
Reason: ユーザーが「判断が必要になったら定期的に質問してほしい。ただし寝ている時は反応できないので、深夜は質問せずに作業してほしい」と指示した。
Impact: COORDINATION.md に「人間への質問(時間帯のルール)」を追加、CLAUDE.md「人間の確認が必要なこと」に1行追加。両レーン・両 AI に適用。

## 2026-09-21: 「TB0 タイプ相性データの取得元・契約を確認待ち」は解決済み(タイプバランスレーン、Claude Code)
Decision: 上記エントリの停止条件は「TB0 のタイプ相性表は差し替え可能な temporary adapter とする」で解除済みであり、TB0 は
その方針で実装・独立レビュー(PASS)・統合した。上記エントリは追記のみの規約により本文を残すが、現状の判断としては読まない。
Reason: TB0 の独立レビューで、未決のまま残った旧エントリが共有状態の読み手を誤解させると指摘されたため。
Impact: なし(記録の整理のみ)。

## 2026-09-21: `.gitignore` の `coverage.*` を `coverage.out` / `coverage.html` に絞る(タイプバランスレーンの提案を採用。データレーン)
Decision: 提案どおり、ルートの `.gitignore` の `coverage.*` を `coverage.out` と `coverage.html` の2行に置き換える。`coverage.*` に依存して無視されていたファイルは無い(`git ls-files -o -i` で確認)。
Reason: `coverage.go` / `coverage.ts` などのソースまで無視され、コミットから黙って漏れる。
Impact: `.gitignore` のみ。

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

## 2026-09-21: 質問の時間帯ルールの重複を1つにまとめ、深夜の PR マージ条件を追加(ユーザー決定)
Decision: 同じユーザー決定を両レーンが別々に COORDINATION.md へ書いたため、先に main に入った「人間への質問(時間帯のルール)」を正とし、
タイプバランスレーンが書いた「判断が必要なときの質問と深夜の自律作業」節と CLAUDE.md の重複行は削除した。
深夜の PR マージは、マージしないと作業が止まる場合に限り可。条件は make test(balance を含む)・lint・build(engine 変更時は test-golden)と独立レビュー PASS、
理由を DECISIONS.md に記録して朝の最初の報告に含めること。
Reason: ユーザーが深夜のマージについて「マージしないと作業止まるならマージしていい」と回答し、条件(テストを通す・朝に報告)を承認した。
Impact: COORDINATION.md「人間への質問」に深夜の PR マージの項を追加。上の「判断が必要なときの質問ルールと深夜の自律作業」エントリは本エントリで置き換える
(深夜の判断待ちの記録先は plan.md のブロッカー節。既定案で進めた判断は DECISIONS.md に「既定案で進行・ユーザー未確認」と書く)。

## 2026-09-21: TB2(攻撃範囲)の仕様3点(ユーザー回答)
Decision: 有効打は等倍以上(×1 以上。抜群は別に数える)。防御側は 18 の単タイプ(複合は TB4)。技はメンバーごとに技 ID を最大4つ送り、タイプ・分類は balance の技の read model から引く。
Reason: TB2 着手時に設計書 §6 で未定義だった点をユーザーに質問し、回答を得た。
Impact: ADR-0016。新 endpoint `/api/balance/v1/team-balance/coverage`、技の read model(`BALANCE_MOVES_PATH`、架空データの example)。

## 2026-09-21: ダメージ計算を3レーン(データ / API / Web)に分け、全体で4レーンを並列に進める(ユーザー決定)
Decision: ダメージ計算レーンを、データ(engine・マスタ・pokedex。`~/MyDamageCalcurater`)、API(calc-svc・gateway・契約テスト。`~/MyDamageCalcurater-api`)、Web(`web/`。`~/MyDamageCalcurater-web`)の3レーンに分ける。タイプバランスと合わせて4レーン。
`api/openapi.yaml` と生成物を変更できるのは API レーンだけ。他のレーンの範囲は変更せず、DECISIONS.md に提案する。待たずに進めるため、暫定の境界(インターフェース・架空データ・fake)を自分のレーン内に置いてよい。
Reason: ユーザーが「Max プランなので、機能単位でもっと並列に起動して実装・レビューしたい」と依頼した。M1 の残りのうち Phase 3(API)と Phase 4(Web)は、engine と WASM が完成済みのため Phase 2 を待たずに始められる。5本以上に分けると openapi.yaml 等の共有ファイルの衝突と利用枠の消費が増えるので4本にした。
Impact: COORDINATION.md(レーン表・依存と共有ファイルの節・起動の目安)と CURRENT_STATE.md(API・Web の欄)を更新。

## 2026-09-21: ルートの .gitignore の `coverage.*` を Go のカバレッジ出力だけに絞る提案(タイプバランスレーンから。既定案)
Decision(提案): `.gitignore` の `coverage.*` は `coverage.go` / `coverage.ts` などのソースも無視してしまう(TB2 で `services/balance/internal/balance/coverage.go` が黙ってコミットから漏れかけた)。
既定案: `coverage.*` を `coverage.out` と `coverage.html`(と各ツールの実際の出力名)に置き換える。持ち主はルートの共有ファイルなのでデータレーンが判断する。
Reason: `git status` に出ないため、テストはローカルで通るのに clone すると壊れる状態になる。Web レーンの `coverage.ts` 等でも起きうる。
Impact: タイプバランスレーンは回避のため本体を `offense.go` にした(変更不要)。他のレーンは、新しいファイルが `git status` に出ることを確かめてから commit する。

## 2026-09-21: iOS レーンを追加して5レーンにする(ユーザー決定)
Decision: M3 の Phase 6(`ios/`)を担当する iOS レーンを新設する(`~/MyDamageCalcurater-ios`、`feat/ios-<phase名>`)。API の契約は `api/openapi.yaml` に追従するだけで変更しない。
サーバーができるまでは生成クライアントに対するモックで作る。Xcode が無い間は Swift Package と `swift test` の範囲で進める。
Reason: ユーザーが「iOS も作りたいので iOS レーンも起動したい」と依頼し、Xcode を導入することにした(導入中)。
Impact: COORDINATION.md のレーン表・依存の節・起動の目安、CURRENT_STATE.md に iOS 欄を追加。準備はタイプバランスレーンのセッションが行った(データレーンの4レーン化の規則に1行ずつ追加しただけ)。


## 2026-09-21: iOS アプリの構成(iOS レーン。既定案で進行・ユーザー未確認)
Decision: ADR-0017。`ios/PokeCalcKit`(Swift Package: 生成 API クライアント・ドメイン/ViewModel・デザイントークン)+ 手書きの `ios/PokeCalc.xcodeproj`(フォルダ同期。View と XCUITest だけ)。
生成物はコミットし `make ios-gen` / `make ios-gen-check`(ルートの `make gen` には入れない)。画面は `PokeCalcService` プロトコルだけを使い、API 実装とモック(架空データ・計算しない)を差し替える。
逆算のドメインは ADR-0010 §R の形にし、`api/openapi.yaml` が P3-1 で更新されるまで API 経由の逆算は「API 未対応」を表示する。構築は契約が無い(P5-4)ので端末内保存の `TeamStore` で作り、Showdown 形式は後回し。配布対象 iOS 26 以上。
Reason: iOS レーンは契約を変更できず、サーバーも未完成。契約の変更を写像1か所で吸収するため。深夜(23 時以降)の着手で質問できないため、取り消しやすい既定案で進めた。
Impact: ルートの Makefile に `include ios/Makefile` の1行、`.gitignore` に SwiftPM の成果物と xcuserdata を追加。

## 2026-09-21: 提案(iOS レーン → API レーン): 構築(team)と逆算の契約
Decision(提案): (1) P3-1 で逆算の契約を ADR-0010 §R8 の形にしたら、iOS は `make ios-gen` と写像の更新で追従する。(2) P5-4 で team-svc の契約を `api/openapi.yaml` に入れるとき、iOS の端末内の `TeamStore`(メンバー: 種族・技・持ち物・特性・性格・SP)と同じ項目を持たせてほしい。
既定案: iOS 側は変更を待たずにモックで進める。
Reason: iOS レーンは `api/openapi.yaml` を変更できない(COORDINATION.md)。
Impact: なし(API レーンの判断待ち)。

## 2026-09-21: 提案(iOS レーン → データレーン): ルート Makefile の help が include したファイルのターゲットを正しく表示しない
Decision(提案): `help` の `grep -E` に `-h` を付ける(複数ファイルのときファイル名が接頭辞になり、`ios/Makefile` 等のターゲット名が表示されない。balance も同じ)。
既定案: データレーンが次に Makefile を触るときに直す。iOS レーンは変更しない。
Impact: 表示だけ。

## 2026-09-21: iOS レーンの既定案の確認(ユーザー回答)
Decision: (1) 構築は team-svc の契約ができるまで端末内に保存(既定案どおり)。(2) Showdown 形式の入出力は後回し(既定案どおり)。
(3) 配布対象は **iOS 27 以上**(既定案の iOS 26 から変更。実機が iOS 27)。(4) API 経由の逆算は P3-1 の契約更新まで「API 未対応」を表示(既定案どおり)。
Reason: ユーザーが「ブロッカーがあれば今答える」と言い、iOS レーンの質問4点に回答した。
Impact: ADR-0017 §1 を iOS 27 に更新。上の「iOS アプリの構成」エントリの未確認の項目は、この回答で確定。

## 2026-09-22: iOS レーンの版を最新の安定版で固定(同日のユーザー決定「言語・ミドルウェア・ライブラリを最新の安定版に」の iOS 分)
Decision: Swift tools 6.4・Swift 6 言語モード・配布対象 iOS 27(macOS 27)・Xcode 27 の推奨設定。依存は swift-openapi-generator 1.13.1 / runtime 1.12.1 / urlsession 1.3.1 / swift-http-types 1.8.0 を `exact` で固定(いずれも確認時点の最新)。
Reason: 方針の本体はデータレーンの DECISIONS エントリ(2026-09-21)。iOS レーンの範囲はデータレーンからの共有による。
Impact: ios/ のみ。

## 2026-09-21: importer(P2-2b)の3点(ユーザー回答。既定案どおり)
Decision: (1) 本番の効果定義 `data/importer/effects.json` を Git にコミットする(英語 ID と 4096 基準の整数だけ)。(2) 日本語名は ja(漢字混じり)を優先し、無ければ ja-Hrkt(かな)。(3) `data/importer/regulations.json` にレギュレーションの日本語ラベルを入れてコミットする。
Reason: ユーザーが確認の質問に回答した。
Impact: ADR-0017 の既定値どおり。plan.md のブロッカーから外した。

## 2026-09-21: 言語・ミドルウェア・ライブラリは最新の安定版に上げ、正確な番号で固定する(ユーザー決定。全レーン)
Decision: Go のツールチェーン、Node、npm パッケージ、Go のモジュール、MySQL・k3s 等のコンテナイメージ、Swift/Xcode 周りを、その時点の最新の安定版に更新し、正確なバージョン(npm は ^ ~ なし、イメージはタグ+できれば digest)で固定する。`latest` 等の自動追従はしない(GitOps で再現できなくなるため)。
古くなったものを一覧にする make ターゲット(例 `make deps-outdated`)を用意し、定期的にまとめて上げる。
@smogon/calc(ゴールデンの照合相手)も新しい版があれば上げる。ただし先に今のゴールデンとの差分を調べてから切り替え(P2-1b と同じ手順)、説明のつかない差分があれば止めて報告する。
分担: 各レーンが自分の範囲の依存を上げる(データ: engine・services/go.mod の共有部分・tools・MySQL・calc / API: API が使うライブラリ / Web: web/ の package.json・Node / タイプバランス: services/balance / iOS: ios/)。Go のツールチェーンの版(go.work・各 go.mod の go/toolchain 行)は全モジュールで揃え、データレーンが先に上げて main に入れ、他のレーンはそれに合わせる。
Reason: ユーザーが「アップデートの手間を減らすため、ミドルウェアやプログラミング言語等のバージョンは全て最新にして」と指示し、固定方法と calc の扱いに回答した。
Impact: 各レーンの次のタスクの前に依存の更新を入れる。更新後はそのレーンのテスト一式を通してから PR にする。

## 2026-09-21: TB3(特性)の仕様3点(ユーザー回答)と細部の既定案
Decision: ユーザー回答: 特性は analyze の request で `abilityId` を任意指定/効果は無効・吸収・倍率変更に加え ×3/4 なども含める/特性による無効・吸収は集計の「無効」に含め source で区別。
既定案(ユーザー未確認): 効果は正規化データ(immune / absorb / type_multiplier / super_effective_multiplier)の read model `BALANCE_ABILITIES_PATH`、
防御の最終倍率は既約分数に広げ(API の multiplier 文字列も "3/4" 等を許す。特性なしなら TB1 と同じ)、category は値の範囲で決める。詳細は ADR-0017。
Reason: TB3 着手時に質問し回答を得た。細部は 23 時以降に決めるため既定案とした。
Impact: ADR-0017。analyze の契約を後方互換で拡張(abilityId 任意、multiplier 文字列の値域拡大、effect の追加、422 unknown_ability、503 条件)。

## 2026-09-21: TB3 の既定案を確認、TB4 の方針、Argo CD の実同期をローカル k3d で行う(ユーザー回答)
Decision: (1) ADR-0017 の既定案(最終倍率は既約分数、category の境界 0/≤1/4/<1/1/<4/≥4)をユーザーが確認。TB3 は今マージしてよい(深夜だがユーザーの指示)。
(2) TB4(仮想敵診断)は、仮想敵を最大 6 体(pokemonId・技 ID 最大 4・特性は任意)で入力し、各仮想敵について自分の各メンバーが受ける最大倍率(受けやすさ)と与えられる最大倍率(打ちやすさ)を表にし、安全に受けられるメンバー数を集計する。TB1〜3 の計算を再利用する。詳細は ADR-0018。
(3) TB0 の Argo CD 実同期は、ローカル k3d に Argo CD を入れ、イメージは k3d のローカルレジストリ、private リポジトリの読み取りはユーザーが作る読み取り専用の fine-grained PAT を、ユーザー自身が ! コマンドでクラスタの Secret に登録する(AI はトークンを見ない・Git に入れない)。
Reason: ユーザーがブロッカーの質問に回答した。
Impact: TB3 を PR で統合。TB4 は ADR-0018 から。Argo CD はタイプバランスレーンの services/balance/deploy/argocd の範囲で進める。

## 2026-09-21: ミドルウェア・ライブラリ・ツールは導入時点の最新の安定版にする(ユーザー決定)
Decision: 各レーンが導入・更新するミドルウェア(Argo CD・DB・メッセージング・監視等)、ライブラリ、ベースイメージ、ツールは、その時点の最新の安定版にする。
再現性のため、版は引き続き完全に固定する(タグ + digest、go.mod・package-lock 等。`latest` タグや範囲指定は使わない)。メジャーバージョンの更新もコードの移行を含めて行う。
ゴールデンの oracle `@smogon/calc` も、新しい版があれば差分を確かめてから上げる(データレーンの同日の記録に合わせる)。Go のツールチェーンの版(go.work と各 go.mod の go/toolchain 行)は全モジュールで揃え、データレーンが先に上げたものに合わせる。
Reason: ユーザーが「ミドルウェア等のバージョンは全て最新にして。アップデートの手間を減らすために」と指示した。
Impact: タイプバランスレーンは Argo CD v3.5.3(2026-09-22 時点の最新。導入手順と版は services/balance/deploy/argocd/README.md・ADR-0018)を導入、balance を Echo v4.15.4 → v5.3.1(oapi-codegen v2.8.0 の echo5-server)に移行、golang:1.27-alpine の digest を更新。
他のレーン(データ・API・Web・iOS)は、自分の範囲の依存を同じ方針で確認・更新する(services/go.mod の Echo v4 は API レーン、services/pokedex/Dockerfile の golang:1.27-alpine の digest はデータレーンの判断)。

## 2026-09-22: データレーンの依存を最新の安定版に固定(ADR-0018)。Go ツールチェーンは 1.27.1、MySQL は LTS(9.7)を既定にする
Decision: (1) Go ツールチェーンを `go.dev/dl` で確認した最新の安定版 `go1.27.1` に統一(`go.work`・`engine`/`services`/`tools`/`services/balance`(go/toolchain 行のみ)の `go` 行)。
(2) engine・services(データレーン分: golang-migrate・go-sql-driver/mysql・yaml.v3)・tools(sqlc)の Go モジュールは `go list -m -u all` と `go mod tidy` で確認したところ既に最新の安定版で変更なし。
services/go.mod の oapi-codegen tool(生成先が API レーンの services/internal/api)は据え置き、上げない(API レーンの担当)。
(3) MySQL は Docker 公式イメージのタグ体系を確認し、現在の LTS 系列は `9.x`(最新 `9.7.2`。旧 LTS の `8.4` は `lts` タグが外れている)、Innovation は年ベースの `26.x` に移行していることを確認。
アップデートの手間を減らす目的に合わせ、Innovation ではなく最新の LTS(`9.7.2`、digest 固定)を既定にした(services/pokedex 周りの statefulset・job-migrate・db-local-up.sh)。
(4) services/pokedex/Dockerfile の `golang:1.27-alpine` を `golang:1.27.1-alpine`(balance と同じ digest)に更新。
(5) `@smogon/calc` は `npm view` で確認したところ `0.12.0` が最新(据え置きどおり変更不要)。`tools/importer/` は `.gitkeep` のみで package.json が無いため対象外(別レーンの未マージ作業)。
(6) 古い依存を一覧化する `make deps-outdated`(Go 各モジュール `go list -m -u all` / Node `npm outdated`)をルート Makefile に追加(`make test` には含めない)。
Reason: 2026-09-21 のユーザー決定(最新の安定版・正確な番号固定・Go ツールチェーンはデータレーンが先に上げる)への対応。
Impact: 詳細は ADR-0018。API レーン・Web レーン・iOS レーンは、Go ツールチェーンを `go 1.27.1` に揃えること(services/go.mod の API レーンが使う分の依存は自分の範囲で確認)。タイプバランスレーンの `services/balance/go.mod` は go/toolchain 行(`go 1.27` → `go 1.27.1`)のみ本コミットで揃え、依存(require)は変更していない。

## 2026-09-22: ADR の番号をレーンごとの帯にする(データレーンの既定案・ユーザー未確認。深夜のため)
Decision: 新しい ADR の番号はレーンごとの帯から取る。データ 0100〜 / API 0200〜 / Web 0300〜 / タイプバランス 0400〜 / iOS 0500〜。既存の 0001〜0019 はそのまま。
衝突している既存の番号は、後から統合する側が自分の帯へ振り直す。データレーンは 0015-pokedex-schema-and-migrate → 0100、依存更新の ADR → 0102、importer の ADR(未統合)→ 0101 に振り直した。
Reason: 5レーンが並行して「main の最新の次」を取った結果、main に 0015 が2つ入り、0016(Web / タイプバランス)・0017(iOS / タイプバランス / データ)・0018(API / タイプバランス)もブランチ間で衝突した。帯にすれば統合の順番に依らず衝突しない。
Impact: COORDINATION.md の共有ファイルの表(docs/adr/)を更新。各レーンは次に ADR を作るときから帯を使い、未統合の ADR が main と衝突していれば自分の帯へ振り直す。深夜のため既定案で進めた(取り消しやすい文書の規則)。朝にユーザーが確認する。

## 2026-09-22: TB5「おすすめタイプと該当ポケモン」を追加(ユーザー要望)
Decision: タイプバランスチェッカーに、チームの穴をふさげるおすすめタイプの候補と、そのタイプを持つ使用可能なポケモン全員の一覧(日本語名付き)を出す機能を TB5 として追加する。
ユーザー回答: おすすめの基準は防御の穴(弱点持ちが多く耐性・無効が少ない攻撃タイプ)と攻撃範囲の穴(有効打が無い防御タイプ)の両方/一覧はそのタイプを持つ使用可能なポケモン全員(特性でふさげるものは別枠)/TB4 を先に作り、TB5 はその後。
Reason: ユーザーが「既存のタイプバランスチェッカーはおすすめタイプは出すが、該当ポケモンを別サイトで探す必要がある。タイプだけ見て候補のポケモンを教えてほしい」と要望した。
Impact: plan.md に TB5。使用可能なポケモンの集合と日本語名はマスタ(データレーンの P2-2。レギュレーション依存)から引く必要がある。それまでは TB1 と同じ temporary の read model を広げる(架空データの example)。


## 2026-09-22: P2-2c の2点と ADR 番号の帯の承認、データレーンは Claude で続ける(ユーザー回答)
Decision: (1) 進化前から継ぐ習得技は、取得データに「学習した世代」の情報を足し、Showdown のチーム検証(M-C は現行世代由来の学習元のみ認める)と同じ判定で絞る(実データの標本 5/5 件が Showdown 本体と食い違ったため)。
(2) P2-1c の裁定の件数・集合が将来の取込で実データと食い違ったら、取り込みを止める(既定案どおり)。
(3) ADR 番号のレーンごとの帯(データ 0100〜 / API 0200〜 / Web 0300〜 / タイプバランス 0400〜 / iOS 0500〜 / 素早さ 0600〜)を承認。
(4) データレーンは Claude で続ける。Claude の上限の間に起動した Codex はデータレーンでは止め、Codex は上限の間の整備だけに使う(ユーザー: 「codex は claude のレートリミットの間だけ整備する作業をさせたかった」)。
Reason: ユーザーが確認の質問に回答した。
Impact: ADR-0103 の習得技の継承を案 b で実装し直す。COORDINATION.md の「Claude の上限時の Codex」の1本目(クリティカルパスのレーン)の扱いは、ユーザーの意図(整備だけ)に合わせて次の文書整理で見直す。

## 2026-09-22: 習得技は進化前から継がない(Champions のルールに合わせる。ユーザー決定。直前の「案 b」を改める)
Decision: 習得技は自分の学習元だけを使い、自分の学習元が無いフォーム・メガだけ基本種の学習元を1段使う。進化前(prevo)からは継がない。
Reason: 案 b(学習した世代で絞る)を実装して実データで確かめたところ、Showdown 本体の検証と 5/5 件食い違った。Showdown のソース(learnsetParent)は champions の mod では進化前をたどらず、実データに9世代より古い学習元は無かった(世代の条件は効果が無かった)。ユーザーが「継承をやめて Showdown に合わせる」を選んだ。
Impact: ADR-0103 §7 を改訂。継承を前提にしたテストは、Champions では継がないことを確かめるテストに書き換える(弱めない)。

## 2026-09-22: API レーンの依頼(内部 API・性格のマスタ・showdownId)を受ける(データレーン)
Decision: P2-3 で pokedex-svc に `GET /internal/pokedex/master`(ADR-0204 の契約。クラスタ内だけ、未投入なら 503)を実装し、species に showdownId を含める。
性格は、ADR-0100 の「マスタにせず engine の固定」を改め、`natures`(id, name_ja, plus, minus)をマスタに加える(新しい migration と importer)。
Reason: calc-svc がマスタを pokedex-svc から受け取る形になり(ユーザー決定 2026-09-22、API レーン)、性格の ID → 補正と日本語名が必要になった。ADR-0013 §2 の「表・一覧はデータ」とも合う。
Impact: plan.md の P2-3 に小項目を追加。ADR-0100 に更新の注記を足す(P2-3 で)。

## 2026-09-22: TB4(仮想敵診断)の仕様(ユーザー回答と既定案)
Decision: 仮想敵を最大 6 体(pokemonId・技 ID 最大 4・特性は任意)で入力し、各仮想敵 × 自分の各メンバーの受ける最大倍率(incoming)と与える最大倍率(outgoing)、
安全に受けられる人数(incoming < 1)・打ちやすい人数(outgoing ≥ 2)を返す。新 endpoint `/api/balance/v1/team-balance/threats`。詳細は ADR-0400。
Reason: ユーザーが TB4 の方針に回答した(「安全に受けられる」「打ちやすい」の閾値はタイプバランスレーンの既定案)。
Impact: TB1〜3 の計算と既存の read model を再利用。

## 2026-09-22: P3-1 calc-svc の API 契約(API レーン。既定案で進行・ユーザー未確認)
Decision: ADR-0200 のとおり。`CalcResult.category` を足す(落とさない)、`BulkCalcRow.defender{sp,nature,natureId,stats}`、逆算は P1-12 の形
(`known` / `unknownSpeciesKey` / `itemCandidates` / 観測は percent・percentTenths・damage のちょうど1つ)、`Error.code` を enum `ErrorCode`
(WASM 境界の語彙 + HTTP だけの missing_header・unknown_*・not_found・master_unavailable・upstream_unavailable)。
マスタは calc-svc 内の暫定 `Store`(services/calc/internal/master)と架空データで作り、データレーンの共通マスタ(P2-2a)が main に入ったら差し替える。
Reason: plan.md P3-1 の小項目と ADR-0010 §9・§R8 / ADR-0011 §10 の持ち越しを解消するため。いずれも取り消しやすい契約の既定値。
Impact: Web レーン(P4-5)は生成型の変更に追従する。データレーンへ: P2-2a の master が入ったら、calc-svc の `Store` インターフェース
(Species / Move / Item / Ability / Nature / NatureID / TypeChart)を満たす adapter を API レーンが作る。

## 2026-09-22: API レーンの依存を最新の安定版へ(上の 2026-09-21「ミドルウェア・ライブラリ・ツールは導入時点の最新の安定版にする」の API レーン分)
Decision: services/go.mod の Echo v4.15.4 → v5.3.1(oapi-codegen v2.8.0 の echo5-server で再生成)、kin-openapi v0.142.0 → v0.149.0、間接依存も最新へ。
例外として go-yit は oapi-codegen v2.8.0 が要求する版に据え置く(最新版は yaml/v4 に移り、make gen が壊れる)。詳細は ADR-0201。Go のツールチェーン行は変えていない(データレーンに合わせる)。
Reason: ユーザー決定の適用。
Impact: services/go.mod はデータレーン(mysql・migrate)と共有。統合時の競合は両方を残して解決した。gateway(P3-2)は最初から Echo v5 で作る。

## 2026-09-22: API レーンの ADR を 0200 台へ振り直し(データレーンが決めた ADR 番号の帯の規則に従う)
Decision: 0018-calc-svc-api-contract → 0200、0019-api-deps-latest-echo-v5 → 0201(タイプバランスの 0018 と衝突していたため)。gateway の ADR は 0202。
Reason: COORDINATION.md の ADR 番号の帯(API は 0200〜)。
Impact: API レーンのファイル(services/calc・api/openapi.yaml・plan.md の P3-1 行・CURRENT_STATE の API 欄・DECISIONS の API レーンのエントリ)の参照だけを置き換えた。

## 2026-09-22: Claude の上限時は Codex を最大2本(クリティカルパスのレーン+整備レーン)で動かす(ユーザー決定)
Decision: Max プランの利用枠は全レーンで共有なので、上限に達すると全レーンが同時に止まる。そのとき Codex を最大2本起動する: (1) M1 の完了に一番効くレーンを通常のプロンプトで Next から続ける、(2) 整備レーン(レーンに属さない共有物の整理・統合の検証・改善要望)。
整備レーンは Claude の各レーンが止まっている間だけ動かし、作業ディレクトリ ~/MyDamageCalcurater-maint は使うときだけ作って終わったら消す。レーンの範囲は直さず、見つけた問題はそのレーンの Next と DECISIONS.md に書く。
Reason: ユーザーが「Codex は Max のレートリミット後に 5 レーンの調整・整備を行うのがよいのでは」と提案し、用意を依頼した。全員が止まっている時間はレーンをまたぐ整理をしても衝突しない。
Impact: COORDINATION.md に節を追加、CURRENT_STATE.md に Maintenance 欄、plan.md に整備レーンのバックログ(MT-1〜7)。

## 2026-09-22: TB5 の詳細は既定案で進める/pokedex export に nameJa・abilityIds とレギュレーションでの絞り込みを依頼(タイプバランスレーン。既定案で進行・ユーザー未確認)
Decision: TB5 は ADR-0401 の既定案(防御の穴 = 耐性・無効が 0 人の攻撃タイプ、攻撃範囲の穴 = 有効打の無い防御タイプ、171 通りの候補を
ふさぐ穴の数で並べ上位 10 件、候補とタイプの集合が一致するポケモン全員、特性でふさげるポケモンは別枠)で作る。
データレーンへの依頼(既定案): ADR-0100 §8 の `pokedex export`(balance 向けのポケモンの read model)で、(1) 各ポケモンに `nameJa` と
`abilityIds`(持ちうる特性 ID。隠れ特性を含む)を出す、(2) 出力するポケモンを既定のレギュレーションの使用可能集合に絞る
(balance は read model に入っているポケモンを使用可能とみなす)。形は ADR-0401 §5(schemaVersion 1 のまま省略可能な項目を追加)。
Reason: 深夜(02 時台)でユーザーに質問しないため。ユーザー回答の3点(基準は両方の穴・使用可能なポケモン全員・特性は別枠)は反映済み。
Impact: 朝にユーザーへ ADR-0401 §2〜§6 の確認を求める。データレーンは export を作るときに上記を含める(異議があれば追記)。

## 2026-09-22: TB5 の既定案をユーザーが確認、該当ポケモンの範囲を変更(ユーザー回答)
Decision: ADR-0401 §2〜§7 の既定案(防御の穴 = 耐性・無効が 0 人、候補の並びと上位 10 件など)をユーザーが確認した。
該当ポケモンの範囲を変更(ADR-0401 §8): 単タイプの候補は、そのタイプを含むポケモン(複合タイプを含む)も並べる。ただし、もう片方のタイプで
候補がふさぐ防御の穴を等倍未満で受けられなくなるものは除く。複合タイプの候補はタイプの集合が一致するものだけ。各ポケモンに `exactMatch` を付け、一致するものを先に並べる。
Reason: ユーザーが 10 時台の確認の質問に「そのタイプを含むポケモンも出す」と回答した(上の「TB5 の詳細は既定案で進める」を置き換える)。
Impact: ADR-0401 §8、OpenAPI の `CandidatePokemon.exactMatch`。

## 2026-09-22: PR #14(API P3-1 calc-svc・依存の最新化)を main に統合
Decision: 他レーンの状況(開いている PR は #14 のみ、Web・iOS の未マージの変更は api/openapi.yaml・services と競合しない)を確認し、main を取り込んで再検証(make test 12 件・lint・build・check-publishable 0 件・make gen 差分なし)してからマージした。
Reason: ユーザーが「テストとか諸々通っているならマージしていい。他のレーンの状況確認してから」と回答した。
Impact: Web(P4-5)・iOS は新しい api/openapi.yaml(category・BulkCalcRow.defender・逆算の新形・ErrorCode)に追従できる。

## 2026-09-22: ADR 番号の帯(データ 0100〜 / API 0200〜 / Web 0300〜 / タイプバランス 0400〜 / iOS 0500〜)をユーザーが承認
Decision: データレーンが深夜に既定案で決めた ADR 番号の帯の規則を、ユーザーが「他レーンと整合するなら承認してOK」と承認した。API レーンは 0200 / 0201 / 0202 を使っており整合している。
Reason: ユーザー回答(API レーンのセッションで受領)。
Impact: COORDINATION.md の規則は確定。gateway の ADR は 0020 → 0202 に振り直した。

## 2026-09-22: check-publishable の自己テストを lint に含める(整備レーン MT-2 の既定案)
Decision: `scripts/check-publishable.sh --self-test` の既知の失敗2件を修正し、`make lint` から `make check-publishable-selftest` を実行する。A は任意のホーム相対パスを検出し、利用者名を含まない共有の worktree と権限定義のプレースホルダだけを許可する。E は自己テストへ実際に禁止される GitHub module path を投入する。
Reason: 検査規則そのものの退行を通常の lint で検出し、自己テストが安全な値を投入して偽陰性になっていた状態を解消するため。plan.md の既定案どおり進めた。
Impact: `make lint` の所要時間が自己テスト分だけ約6秒増える。A〜F の違反検出・値の非表示・正常系を毎回確認する。

## 2026-09-22: 整備レーン MT-1 / MT-2 を PR #24 で main に統合
Decision: 最新 main の統合検証(MT-1)と check-publishable 自己テストの修正・lint 組み込み(MT-2)を PR #24 で main に統合した。MT-2 の独立レビューは PASS(重大・重要・軽微 0件)。
Reason: 必須の test・lint・build・公開前検査、および golden・全種族・WASM・balance の非クラスタ検証が成功したため。
Impact: 整備レーンの次回開始点は MT-3。データ・API・Web・タイプバランス各レーンの再開を確認したため、本 worktree は削除する。

## 2026-09-22: iOS の ADR を 0500 に振り直し(レーンごとの番号帯。データレーンの規則に従う)
Decision: `docs/adr/0017-ios-app-architecture.md` を `docs/adr/0500-ios-app-architecture.md`(ADR-0500)に改名し、ios/・plan.md の M3 節・CURRENT_STATE.md の iOS 欄の参照を更新した。
上の iOS のエントリ(2026-09-21)に書いた「ADR-0017」は iOS の構成の ADR のことで、以後は ADR-0500 と読む(main の ADR-0017 は balance TB3)。
Reason: main の ADR-0017(balance TB3)と番号が衝突した。後から統合する側(iOS)が振り直す(COORDINATION.md)。
Impact: ios/ と文書の参照のみ。

## 2026-09-22: iOS の逆算画面の観測入力と PR の区切り(ユーザー回答)
Decision: (1) 逆算の観測(与えたダメージ = 相手 HP の減少%(整数)、受けたダメージ = 自分 HP の減少量(実点数))はテンキーで数値入力する
(requirements.md「数値の直接入力は原則しない」の例外。観測値は選択肢から選べないため)。(2) main への PR は、P6-2 の契約追従が緑になった時点で
P6-1・P6-2a・契約追従をまとめて出す。逆算・構築は次の PR。
Reason: 日中にユーザーへ質問し、既定案(推奨)どおりの回答を得た。
Impact: P6-2b の画面仕様、PR の区切り。
## 2026-09-22: 素早さ比較を3つ目のサービスとして新しいレーン(6本目)で作る(ユーザー決定)
Decision: 素早さ比較サービス(`services/speed/`)を新しい「素早さ」レーンで作る(`~/MyDamageCalcurater-speed`、ブランチ `feat/speed-<stage名>`、ADR は `0600〜`)。
画面は Web に独立したタブ。素早さの画面は `web/src/speed/` を素早さレーンの持ち物にし、アプリの骨組み(タブの登録)は自分の1項目を足すだけにする。
仕様(ユーザー回答): 左 = 速い順の全体の表、右 = 自分のポケモン、自分の位置を視覚的に示す。表は各ポケモン6行(無振り / 準速 / 最速 / 最速スカーフ / 最速+1 / 最速+2)。右は「無振り / 準速 / 最速」+スカーフ on/off の最小の選択で計算でき、オプションで好きな数値でも算出できる。
Reason: ユーザーが素早さ比較サイトの使い勝手(表と見比べて自分の数値を算出する)を改善したいと依頼し、表の行・入力・担当(タイプバランスの次ではなく新しいレーン)・画面の置き場所に回答した。
Impact: COORDINATION.md のレーン表・ADR の帯・起動の目安、CURRENT_STATE.md の Speed 欄、plan.md の「SP: 素早さ比較」(SP0〜SP4)。データレーンの P2-3 の read model(`pokedex export`)に、素早さの種族値が含まれていること(ポケモンの read model に baseStats があれば足りる)。

## 2026-09-22: 素早さ比較の未確定4点をユーザーが回答(素早さレーン)
Decision: (1) 右のオプションは SP 0〜32・性格の補正3通り・ランク -6〜+6・スカーフ、または実数値の直接入力 (2) 同じ実数値は同速としてまとめて表示 (3) 表は既定のレギュレーションの使用可能集合 (4) `web/` の骨組みが無い間は `web/src/speed/` の画面部品とテストだけ先に作り、タブ登録は骨組みができてから1項目足す。いずれも既定案どおり。
Reason: 素早さレーンの着手時に AskUserQuestion で確認した。
Impact: docs/plan.md「SP: 素早さ比較」の未確定を確定に更新。docs/speed-design.md・ADR-0600 に反映。

## 2026-09-22: 素早さ SP0 の設計(素早さレーンの判断)
Decision: services/speed は engine に `replace` で依存し、実数値・ランクは `engine.RealStats` / `engine.EffectiveStat` を呼ぶ。engine に無いこだわりスカーフ(×1.5)だけを speed のコアが 4096 基準の補正 6144・五捨五超入で持つ(ランクの後。Showdown の順)。GitOps の overlay と Argo CD Application は、イメージの digest が決まる SP4 で作る(ADR-0600)。
提案(データレーンへ。既定案: 今は何もしない): engine に素早さの持ち物補正(スカーフ)の公開関数を足すなら、speed はそれを呼ぶように切り替える。足さない場合は speed の1式のままでよい。
Reason: engine はデータレーンの範囲で、Champions に無い効果をダメージ計算の engine に入れない方針のため。表の行としてスカーフはユーザーの仕様で必要。
Impact: ADR-0600、docs/speed-design.md。

## 2026-09-21: Web レーンの構成(ADR-0300)と、他レーンへの提案2件(Web レーン、Claude Code。既定案で進行・ユーザー未確認)
Decision: (1) Web は計算を `CalcEngine` の後ろに置き、WASM(ADR-0011 の JSON 契約)で先に作る。マスタは `MasterData` の後ろに置き、
pokedex-svc ができるまで架空の例データ(名前は `テスト`、ID は `example-`、図鑑番号 9001 以降)。タイプ相性表だけは
`testdata/golden/typechart.json` を Vite の別名で複製せずに読む(Web は読むだけで変更しない)。
(2) **提案(データレーン宛て)**: 攻撃側プリセット(無振り / A(C)特化 / A(C)振り。ADR-0300 §5)は、いまは Web が持つ。
防御プリセット(ADR-0009)と同じく engine が持つ方が一貫し、iOS(M3)も同じ定義を使うので、既定案は
「データレーンが `AttackerPresetCatalog()` を engine と WASM 境界に足し、Web はそれに切り替える」。急がない(M3 より前ならよい)。
(3) **提案(全レーン宛て)**: Web のテスト(`make web-test` / `web-lint`)は、まだ `make test` / `make lint` に含めない
(含めると `web/node_modules` の無い他のレーンの作業ディレクトリでルートの `make test` が失敗する)。
既定案は「P4-6 で、`node_modules` が無ければ `npm ci` してから実行する形で `make test` / `make lint` に加える」。
Reason: 4レーン制で API・データの成果を待たずに Web を進めるため(COORDINATION.md「レーン間の依存と共有ファイル」)。
Impact: ルートの Makefile には `include web/Makefile` の1行だけを足した(ターゲットは `web-` 接頭辞)。engine・openapi.yaml は変更しない。
docs/design.md に bg.glass のぼかし量(Web は 20px。iOS はシステムのマテリアル)を1行追記した(P4-1)。
異議があれば追記すること(既定案で進む原則)。

## 2026-09-21: 言語・ミドルウェア・依存のバージョンは最新にする(ユーザー決定。Web レーンのセッションで受領)
Decision: 「アップデートの手間を減らすため、ミドルウェアやプログラミング言語等のバージョンは全て最新にする」。新しく入れる依存・イメージ・ツールは
その時点の最新の安定版を選び、完全固定(lockfile・digest)は従来どおり続ける。互換性の都合で最新にできないものは、理由と追従の条件を ADR か本ファイルに書く。
Web レーンの反映: TypeScript を 7.0.2 に上げる(型検査は TS 7 のネイティブ版)。typescript-eslint 8.70 は TS 7 の API に未対応のため、
ESLint が読む `typescript` だけ公式の互換パッケージ `@typescript/typescript6` を別名で入れる(typescript-eslint が TS 7 に対応したら外す)。他の依存は確認時点で最新。
確認時点の最新: Go 1.27.1(go.mod は 1.27 で最新)、Node.js 26.9.0(この Mac を 26.4.0 から上げ、`web/.node-version` と `web/package.json` の `engines` で固定)。
Reason: ユーザー指示(2026-09-21)。
Impact: 各レーンは自分の範囲の依存・イメージ(MySQL・TiDB・NATS・k3d 等を含む)を次の区切りで最新に揃える。他レーンのファイルは各レーンが変更する。

## 2026-09-22: Web P4-5 の方針(ADR-0301)と、API レーンへの連絡(Web レーン、Claude Code。既定案で進行・ユーザー未確認。深夜)
Decision: (1) 画面は解決済みの実体のまま、API 実装が実体 → ID に写す(解決層は MasterData の1か所)。写像の表は ADR-0301 §2。
(2) 計算モードの既定はオフライン(WASM)。オンラインは pokedex-svc(P2-3)と gateway(P3-2)が揃ってマスタを API から読めるようになったら既定を見直す。
API に届かないとき自動で WASM に切り替えない(どちらの結果か分からなくなるため)。
(3) Web の例データの種族キーを `SpeciesKey`(`9001-000` の形)に合わせる(ADR-0300 §3 を改める)。
(4) **API レーンへ**: ルート Makefile の `gen-ts` を実装した(`web/src/api/openapi.gen.ts` を生成してコミット)。`make gen` に含まれるので、
`api/openapi.yaml` を変えたら一度 `make web-install` してから `make gen` する(web の依存が無いと gen-ts は失敗する。スキップしない)。
Reason: ADR-0011 §10 の持ち越し(P4-5)。API の契約(ADR-0200)が main に入ったため。
Impact: 他レーンのファイルは変更しない(gen-ts は Web の持ち物のターゲット)。

## 2026-09-22: Web レーンの確認事項4件(ユーザー回答)と PR #22 の統合
Decision: (1) PR #22(Web P4-1〜P4-5)を main にマージする。(2) Web のテスト(web-test・web-lint)をルートの `make test` / `make lint` に含める。
`web/node_modules` が無ければ `npm ci` してから実行する(P4-6 で実装)。(3) 計算モードの既定はオフライン(WASM)のまま(ADR-0301 §4)。
pokedex-svc と gateway が揃ったら見直す。(4) `gen-ts` は web の依存が無ければ失敗させる(ADR-0301 §7)。API レーンは `make gen` の前に一度 `make web-install`。
Reason: 夜の間に既定案で進めた判断を、朝の最初の区切りでまとめて確認した(COORDINATION.md「人間への質問」)。
Impact: (2) により、他のレーンのルートの `make test` / `make lint` でも Web のテストが走る(初回は npm ci の分だけ遅い)。

## 2026-09-22: Claude の上限時の Codex は整備レーンだけにする(ユーザー決定。前エントリの「最大2本」を改める)
Decision: Claude の上限時に Codex で進めるのは整備レーンだけ。Codex はレーン(データ・API・Web・タイプバランス・iOS・素早さ)の作業を引き継がない。
Reason: ユーザーが「Codex は Claude のレートリミットの間だけ整備する作業をさせたかった」と述べた。上限の間に Codex がデータレーンを引き継いだ結果、Claude の再開後に同じディレクトリで2つの AI が動く状態になった(データレーンの Codex はユーザーの指示で停止)。
Impact: COORDINATION.md の「Claude の上限時の Codex」を整備レーンだけに改訂。

## 2026-09-22: Web の E2E(P4-6)とルートの make への組み込み(Web レーン、Claude Code)
Decision: (1) ユーザー決定どおり、`web/Makefile` が `test: web-test` / `lint: web-lint` / `build: web-build` を足した。依存が無い・lockfile より古いときは自動で `npm ci`。
(2) E2E は `make web-e2e`(オフライン = WASM。`/api` を遮断しても計算できること、engine.wasm は初回の計算まで読まないこと)と
`make web-e2e-online`(例データを書き出して calc-svc を `go run` で起動し、オンラインとオフラインの結果の行が一致すること)。chromium のみ。
(3) **提案(API レーン宛て、既定案)**: ルートの `make e2e`(`scripts/e2e.sh`。P3-3 の k3d スモーク)の最後で `make web-e2e` を呼ぶ。
gateway 経由のオンライン E2E(`VITE_API_BASE_URL` を gateway に向ける)は P3-3 の後に Web レーンが足す。
Reason: P4-6 の完了条件と、ユーザー回答(2026-09-22)の反映。
Impact: 他のレーンのルートの `make test` / `lint` / `build` で Web のテスト・型検査・ビルドも走る(初回は npm ci ぶん遅い)。
## 2026-09-22: PR #23(API P3-2 gateway)を main に統合(深夜。ユーザーの指示に基づく)
Decision: critic PASS(NG 2回のあと3回目)、make test 14 件・lint・build・check-publishable 0 件・make gen 差分なし、開いている他の PR 無し・Web/iOS/データのブランチに競合する変更無し、を確認してマージした。
Reason: ユーザーが同日「PR はテストとか諸々通っているならマージしていい。他のレーンの状況確認してから」と指示した(深夜のマージ条件より優先)。
Impact: Web(P4-5)・iOS は gateway 経由の API(ErrorCode に invalid_header、X-Device-Id / X-Session-Id は UUID)に追従する。

## 2026-09-22: 提案(データレーン・整備レーンへ): `make up` 後の calc・gateway のイメージ(API レーン。既定案)
Decision: P3-3 で共有の local overlay に `components: [api]` を足したため、`make up`(scripts/up.sh)も calc・gateway の Deployment を作るようになる。
up.sh は `pokecalc/calc:local` / `pokecalc/gateway:local` をビルド・import しないので、`make api-k3d-deploy` を流すまで ImagePullBackOff のまま残る。
既定案: 手順として `make up && make api-k3d-deploy` を README(services/gateway/README.md)に明記する(API レーンで実施済み)。
up.sh の最後で `make api-docker-build` と `k3d image import` を呼ぶ形にするかは、up.sh の持ち主(データレーン・整備レーン)の判断に任せる。API レーンは scripts/up.sh を変えない。
Reason: critic の推奨。共有スクリプトは他レーンの範囲のため。
Impact: `api-k3d-deploy` は他レーンのリソースに触れないよう、常に API 専用の overlay(deploy/k8s/overlays/local-api)だけを適用する(ADR-0203)。

## 2026-09-22: iOS レーンの統合(PR #31)
Decision: P6-1(ADR-0500)・P6-2a 計算画面・P3-1/P3-2 の契約変更への追従を PR #31 で main にマージした(critic はそれぞれ PASS。make test / lint / build / check-publishable / ios-test が成功)。
Reason: ユーザー回答(2026-09-22)「契約追従が緑になったら PR」。
Impact: 続き(P6-2b 逆算画面・P6-2c 構築)は同じブランチ feat/ios-p6 で進める。

## 2026-09-22: iOS の API 生成は pokedex・calc タグだけにする(API レーンからの連絡への回答)
Decision: `ios/tools/openapi-gen/openapi-generator-config.yaml` に `filter.tags: [pokedex, calc]` を入れ、`internal` タグ(`GET /internal/pokedex/master`。ADR-0204)を iOS の生成物に含めない。
feat/api-master-adapter の api/openapi.yaml でも生成物がいまと同一になることを確認した(internal の型は出ない)。
Reason: サーバー間の API で、gateway も公開せずアプリは呼ばない。生成すると使わない型が増え、internal の変更のたびに iOS の生成物がずれる。
Impact: iOS がアプリで新しいタグ(例: 構築の team)を使うときは、この設定に足してから `make ios-gen` する。

## 2026-09-22: 素早さ SP0 を PR #32 で main に統合(素早さレーン)
Decision: SP0(ADR-0600)を PR #32 で統合した。critic は1回目 NG(smoke の架空名)→ 修正後 PASS。make test・lint・build・check-publishable・smoke が成功。
Impact: 素早さレーンは SP1(feat/speed-s1)へ。

## 2026-09-22: Web の統合記録(PR #22・#28)
Decision: PR #22(Web P4-1〜P4-5)と PR #28(P4-6 Playwright E2E・make test への Web の組み込み・verify-m1.md ドラフト)を main に統合した。
Reason: 独立レビュー(critic)PASS と、make test / lint / build・E2E の通過を確認した後(COORDINATION.md「main への統合」)。
Impact: 残りは P4-5 のブラウザ実機確認(人間)と P4-7 の完成(P2-2c/d・P2-3・P3-3 を待つ)。

## 2026-09-22: 逆算の表示は型でまとめない/次は design.md の演出(P4-8)(ユーザー回答)
Decision: (1) 逆算の候補は engine の順に1件ずつカード表示し、目安の型の名前を併記する(型でまとめない)。design.md「画面: 逆算」を改めた。
(2) Web レーンの次の作業は design.md「動き」の演出(P4-8。操作時のみ、視差効果を減らす設定で無効)。
Reason: ユーザー回答。逆算の結果(性格 × 持ち物ごとの SP 範囲)では型が一意に決まらないため。
Impact: design.md の1行、plan.md に P4-8、ADR-0300 §7 の持ち越しの記述を更新。iOS(M3)も同じ表示方針に従う。

## 2026-09-22: マスタの定期取込(P2-2d)の2点(ユーザー回答。既定案どおり)
Decision: (1) 取得元に新しい版が出ていても CronJob は成功のまま、ログと報告で知らせるだけにする(取り込むのは Git に固定した版だけ。版を上げるのは人が PR で config.json を更新する)。(2) 実行は毎週土曜 12:00(日本時間)。
Reason: ユーザーが確認の質問に回答した。
Impact: ADR-0104 の既定値どおり。

## 2026-09-22: Web P4-8 を統合(PR #33)
Decision: P4-8(design.md「動き」の演出)と逆算の表示方針・design.md の演出の値を PR #33 で main に統合した。Web レーンは他レーン(P2-3・P3-3)待ちで一時停止。
Reason: critic PASS、make test / lint / build・E2E の通過を確認。
Impact: Web レーンの Active を「なし」にした。続きは CURRENT_STATE.md の Web 欄の Next。

## 2026-09-22: 素早さ SP1 を PR #36 で main に統合(素早さレーン)
Decision: SP1(ADR-0601。表の 6 行・速い順・同速の段・presets の絞り込み)を PR #36 で統合した。critic PASS(軽微4。テストのコメントは修正、空の roster の扱いは SP4 までに決める)。
Impact: 素早さレーンの次は SP2(feat/speed-s2)。
## 2026-09-22: 手順書の書き方を全レーン共通のルールにする(ユーザー決定。Web レーンのセッションで受領)
Decision: 人が実行する手順書は、上から下へ1回読めば終わる形にし(節の間を行き来させない)、動作を伴うコマンドと必要最低限の確認点だけを書く
(行動を伴わない説明は ADR や設計の節へ)。コマンドの塊はリポジトリのルートへの `cd` から始め、ローカルの手順は k3d(コンテナ)を主にする。
AGENTS.md に「手順書の書き方」節を追加し、CLAUDE.md の「最初に読むもの」から参照した。
Reason: ユーザーが「手順書を上下に行き来するのは手間」「行動を伴わない説明は要らない」「make の実行場所で迷う」「全レーンに共有して」と指示した。
Impact: 全レーン・両 AI に適用。既存の手順書は、各レーンが次に触るときにこの形に直す(Web は docs/verify-m1.md を P4-14 で直す)。

## 2026-09-22: README・手順書・構成図の規則(ユーザー決定。全レーン)
Decision: 各コンポーネントに README(何をするか・mermaid の構成図・ディレクトリ・コマンド・関連 ADR。80 行以内)、動かせるレーンには手順書 `docs/runbooks/<レーン>.md`、全体図は `docs/architecture.md`。
手順書の書き方は AGENTS.md「手順書の書き方」(Web レーンが PR #38 で追加した全レーン共通の規則)に従う。
図は mermaid を基本にする。文書は短く、重複させずリンクでつなぐ(読んで直すのは人間。量が多いと疲れる)。
Reason: ユーザーが「各 README で何をしているか・どうしているかの説明、動作確認の手順書、アーキテクチャの図が欲しい。人間が後で読みやすく AI も扱いやすく、ただし過剰な量にしない」と依頼した。
Impact: docs/coding-rules.md §8、docs/architecture.md(全体図)、plan.md の「DOC: 文書」(各レーンのタスク)。各レーンは自分の範囲の README・手順書を書く。
## 2026-09-22: calc-svc のマスタを pokedex-svc の内部 API から受け取る(ユーザー決定。ADR-0204)
Decision: calc-svc のマスタの入手元を pokedex-svc の内部 API `GET /internal/pokedex/master`(契約は api/openapi.yaml の tag `internal`、operationId `getMasterExport`、200 は MasterExport、503 は master_unavailable)にする。
calc-svc は起動時に取得し(失敗は指数バックオフで再試行、取得後は再取得しない、更新は再起動で反映)、services/internal/master の写像でメモリに載せる。取得できるまで計算は 503 master_unavailable、readiness(/readyz)も 503。
gateway は /internal/* を公開しない。k3d local と `make dev` は同じ形の JSON ファイル(架空データ)で動かす。
Reason: ユーザーが「pokedex-svc の内部 API」を選んだ(絶対ルール4を守り、マスタの正本を pokedex の DB 1つにするため)。
Impact(データレーンへの提案。既定案): (1) pokedex-svc(P2-3)で `GET /internal/pokedex/master` を実装する(クラスタ内の Service だけで Ingress には出さない。DB に未投入なら 503 master_unavailable。使用可能集合で絞らない。effect は item_effects / ability_effects の JSON をそのまま返す)。
(2) natures テーブル(id, name_ja, plus, minus)を追加し、/api/pokedex/natures と内部 API の両方で使う。(3) MasterExport の species には showdownId を含める(共通マスタの Species が形式を検証するため。nameEn は含めない)。
API レーンの後続: pokedex-svc のデプロイ後に calc の local overlay を URL 方式(`CALC_MASTER_URL=http://pokedex`)に切り替える。
Web / iOS へ: openapi に tag `internal` の操作と Master* の型が増える(web/src/api/openapi.gen.ts はこの PR で再生成済み)。iOS は生成し直すか、生成設定で `internal` タグを除外する。

## 2026-09-22: 無効・吸収の特性は後続タスク P2-3b で engine と効果定義に足す(ユーザー決定)
Decision: ふゆう・ちょすい等の「特定のタイプの技を無効・吸収する特性」は、P2-3 には入れず、後続の P2-3b で engine の効果定義(AbilityEffect)・DB・importer・export に足す。ダメージ計算でも 0 になり、ゴールデンと照合する。
Reason: 今の効果定義に無いため、タイプバランスの判定にもダメージ計算にも反映されていない。ユーザーが「後続タスクで足す」を選んだ。
Impact: plan.md に P2-3b。タイプバランスレーンは P2-3b が入るまで、タイプ由来の相性と倍率を変える特性だけで判断する。

## 2026-09-22: PR #30(API P3-3)・PR #42(API P3-4 マスタを pokedex-svc の内部 API から)を main に統合
Decision: どちらも critic PASS、make test・lint・build・check-publishable 0 件・api-kustomize・make gen 差分なし、k3d の api-smoke 成功、開いている他の PR と未マージのブランチとの重なりが無いこと(#42 のときは #39 がドキュメントのみ)を確認してマージした。
Reason: ユーザーの指示(テストが通り他レーンを確認済みならマージしてよい)。
Impact: API レーンの Phase 3 と P3-4 は完了。次は Web レーンの依頼(GATEWAY_WEB_URL)と DOC-api。


## 2026-09-22: Web P4-9 を統合(PR #35)
Decision: P4-9(P4-8 の軽微な改善3件)を PR #35 で main に統合した。Web レーンは他レーン(P2-3・P3-3)待ちで一時停止(Active: なし)。
Reason: critic PASS、make test / lint / build・E2E の通過を確認。
Impact: 続きは CURRENT_STATE.md の Web 欄の Next。

## 2026-09-22: Web の画面・コンテナ化・手順書の方針(ユーザー回答)と、API レーンへの依頼
Decision: (1) 画面は URL で切り替える(`/calc`・`/reverse`。P4-10)。(2) タイプバランスと素早さ比較の画面を Web に作る(P4-12・P4-13。balance / speed API を使う。
各サービスのレーンの API 契約は変えずに使う)。(3) ローカルでもコンテナ(k3d)で動かすのを主にする。Web は nginx の静的配信イメージにし、
**gateway の後ろ**に置く(localhost:8080 だけで画面も API も使える。P4-11)。(4) 手順書は上から順に実行するだけで済む形にし、各コマンドは
リポジトリのルートへの `cd` から始める(P4-14)。(5) P4-5 のブラウザ実機確認は Chrome で良好(Safari は未確認)。
**依頼(API レーン宛て)**: gateway に任意の `GATEWAY_WEB_URL` を足し、設定されていれば `/api`・`/assets`・`/healthz` 以外のパスを Web(nginx の Service)へ転送してほしい
(未設定なら従来どおり)。Web レーンは base/web の Service 名 `web`(port 80)を用意する。それまでは `kubectl port-forward` で Web を開く。
Reason: ユーザーが make web-dev で確認したうえで「他の画面も見たい」「make の実行場所で迷う」「ローカルもコンテナで動かして k8s の恩恵を受けたい」「手順書を上下に行き来する」と要望した。
Impact: plan.md に P4-10〜P4-14。gateway の変更は API レーンの範囲なので Web レーンは変更しない。

## 2026-09-22: 素早さの画面は素早さレーン(SP3)のまま(ユーザー決定)
Decision: 素早さ比較の画面は、COORDINATION.md のとおり素早さレーンの SP3(`web/src/speed/`)が作る。Web レーンの P4-13 は取り消す。
Web レーンは P4-10 の URL で画面を切り替える仕組み(ルート表)を、素早さレーンが1項目足すだけで `/speed` を登録できる形にする。
タイプバランスの画面(P4-12)は、担当が決まっていないので Web レーンが作る。
Reason: 上の「Web の画面・コンテナ化・手順書の方針」で P4-13 を Web に置いたが、素早さの画面は既に素早さレーンの範囲と決まっていた(素早さレーンの指摘)。ユーザーが素早さレーンのままを選んだ。
Impact: plan.md の P4-13 を取り消し。素早さレーンの SP3 はそのまま。

## 2026-09-22: pokedex export の abilityIds 上限を 4 に、実データの配線をタイプバランスレーンが実装(データレーンの依頼への回答)
Decision: データレーンの依頼(abilityIds を4件に、export の read model を balance に読ませる配線)を受け、ADR-0401 §5(上限 4)と ADR-0403(配線)で実装した。
`make balance-k3d-deploy-readmodel` / `make balance-smoke-readmodel`(docs/runbooks/balance.md 2b)で、data/generated/readmodel/ の実データを検証してから k3d の balance にマウントする。
Reason: データレーンからの依頼(2026-09-22)。
Impact: pokedex export はそのまま出力してよい(slot 4 を落とさなくてよい)。無効・吸収の特性が export に無いことは了解済みで、当面は倍率を変える特性だけ反映される。
## 2026-09-22: 各レーンのメインセッションは Sonnet で起動する(ユーザー決定)
Decision: Claude Code の各レーンのメインセッションは `--model sonnet` で起動し、設計の判断が重いときだけ `/model opus` に切り替えて戻す。サブエージェントの割り当て(spec-writer・critic は Opus、implementer は Sonnet、quick-scanner は Haiku)は変えない。
Reason: 6レーンのメインセッションをすべて Opus で動かすと、Max プランでも5時間の利用枠に達する。ユーザーが「メインだけ Sonnet にする」を選んだ。
Impact: CLAUDE.md のワークフロー、COORDINATION.md の起動の目安。動いているセッションは `/model sonnet` で切り替える。

## 2026-09-22: サブエージェントも重い作業のときだけ Opus にする(ユーザー決定。前エントリ「メインだけ Sonnet」を改める)
Decision: メインセッションは Sonnet で起動し、重い設計の判断のときだけ Opus。spec-writer・critic は engine・逆算・DB・API 契約に関わるときだけ Opus(既定)、文書・k8s・スクリプト・軽い修正では Sonnet で呼ぶ。利用枠が厳しいときは M1 のレーン(データ・API・Web)を優先し、他のレーンは区切りで止める。
Reason: ユーザーが確認の質問に改めて答えた(前回の回答「メインだけ Sonnet」は意図と違った)。
Impact: CLAUDE.md・COORDINATION.md を更新。
