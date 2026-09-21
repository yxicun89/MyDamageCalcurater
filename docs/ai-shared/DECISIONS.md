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

