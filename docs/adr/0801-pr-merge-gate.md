# ADR-0801: テストと CI が通った PR は AI がマージしてよい(検証ゲート付きの scripts/pr-merge.sh)

- 状態: 採用(2026-10-03。ユーザー決定。「全レーンでテストと CI が通っていればマージしてよい。マージのたびに作業が
  止まるのが負担。止めてよいのは、クラウドへの勝手なデプロイなど費用が発生すること、機密情報を Git で公開することだけ」)
- 日付: 2026-10-03
- 関連: ADR-0800(AI の許可設定と PreToolUse ガード。`gh pr merge` を常に止めていた)、CLAUDE.md「人間の確認が必要なこと」、
  docs/ai-shared/COORDINATION.md「main への統合(PR)」、ADR-0119(CI)

## 背景
ADR-0800 は、意図しない main への反映を防ぐため、素の `gh pr merge`(と `gh api` 経由のマージ)を bash-guard で常に止め、
人間が自分の端末でマージする運用にした。これにより、検証済みの PR ごとに AI の作業が止まり、全レーンで待ち時間が積み上がった。
ユーザーは、止めるべきなのは「費用の発生」と「機密の公開」だけで、テスト・CI が通った PR のマージまでは止めたくないと決めた。

## 決定

### 1. マージは検証ゲート付きの `scripts/pr-merge.sh <PR番号>` だけを通す
素の `gh pr merge`・`gh api` でのマージは、これまでどおり bash-guard が止める(ADR-0800 §2 を変更しない)。許可するのは
`scripts/pr-merge.sh` の呼び出しだけで、`.claude/settings.json` の `permissions.allow` に `Bash(scripts/pr-merge.sh *)` を足す。
ゲート(すべて満たすときだけマージする。1つでも満たさなければ非0で終了し、マージしない):

1. PR が OPEN・ドラフトでない・コンフリクトが無い。
2. GitHub の checks が1件でも在れば、すべて成功(失敗・実行中・保留があれば中止)。1件も無い(CI が走っていない。
   2026-10-03 時点の実態)ときは、次のローカル検証が CI の代わり。
3. 保護ファイルを変更する PR は対象外(人間がマージする): `.claude/`・`.codex/`・`scripts/ai-guard/`・`scripts/pr-merge*.sh`
   (AI がゲート自身や権限を緩めて通すことを防ぐ)、`deploy/k8s/overlays/cloud/`・`terraform/`・`.github/workflows/`
   (クラウドへのデプロイ・費用に関わる設定)。
4. PR の先頭コミットを使い捨ての worktree に取り出し、`make lint`・`make check-publishable`・`make test` を通す。
   `engine/`・`testdata/golden/` を変える PR は `make test-golden` も。機密・実データ・個人情報の混入は
   `check-publishable` が検査する(ユーザーが止めたい「機密の公開」の自動検査)。
5. マージは `--merge --match-head-commit <検証した SHA>`。検証後に PR が更新されたらマージされない。`--admin` は使わない。

### 2. 止めたままのもの(ユーザーが止めたいと言った領域)
- **費用の発生**: クラウドへのデプロイ・クラウドリソースの作成(保護ファイルの人間マージ、ADR-0210 の「クラウド公開はしない」、
  CLAUDE.md の「人間の確認が必要なこと」)。AI の権限設定・ガード自体の変更も人間がマージする。
- **機密の公開**: `check-publishable` が落ちる PR はマージされない。実 Pokémon データ・生成済みスナップショット・公式画像・
  資格情報・開発機の IP 等を Git に入れない(ADR-0002・CLAUDE.md)。
- main への直接 push・force push は従来どおり deny(変更なし)。

### 3. 各レーンの手順
COORDINATION.md の統合手順の最後を、素のマージコマンドから `scripts/pr-merge.sh <番号>` に置き換える。ゲートを通らないときは、
理由(stderr)に従って直してから再実行する。ゲート自体が通らない構造的な理由(保護ファイル等)のときは、そのままにして次の作業へ進む
(人間のマージ待ち。作業は止めない)。`--check` を付けるとマージせずゲートだけを確かめられる。

## 却下した案
- **allow に `gh pr merge` を戻す**: テスト・CI の結果にかかわらずマージできてしまい、ゲートが無い。
- **GitHub のブランチ保護(必須 checks)だけに頼る**: 現状 CI が走っておらず(checks が 0 件)、保護の条件を満たせない。
  CI が安定して走るようになれば、ゲートの 2 を必須化する(別 ADR)。
- **環境変数でゲートを省略できるようにする**: AI が省略して通せてしまうので、省略の手段は作らない。

## 影響・限界
- ローカル検証は PR の先頭コミットで行う。iOS の `make ios-test`・Playwright e2e・k3d への実適用は含まない(CI 同様。
  CI の対象外と同じ理由)。それらを必要とする変更(iOS・e2e)は、PR 本文に実施した検証を書く。
- 実行時間が長くなる(`make test` 一式)。`--check` で先に確かめ、ゲートの結果を再利用するキャッシュは持たない(単純さ優先)。
- ゲートは「AI が誤って壊す」ことを防ぐもので、悪意ある AI への防御ではない(ADR-0800 と同じ立場)。

## 追記(2026-10-03): iOS のゲート(ユーザー決定。「iOS レーンで、テストが通って条件を満たした PR はマージできるように」)
- **背景**: ゲートのローカル検証の `make test` は Go 専用(`engine`・`services`)で、iOS の XCTest・XCUITest を含まない。iOS だけを変える PR は
  iOS のテストを一度も通さずにマージできてしまう。iOS レーンの完了条件は `make ios-test`(ios/README.md・COORDINATION.md)。
- **決定**: ゲートの手順 4 に続けて、**`ios/` または OpenAPI 契約(`api/openapi.yaml`・`services/*/api/openapi.yaml`)を変える PR は
  `make ios-test` も通す**(lint・生成物の一致・件数上限の同期・XCTest・XCUITest・Info.plist 検査)。契約を含めるのは、
  契約の変更が iOS の生成物(`ios-gen-check`)を壊しうるため。Xcode(`xcodebuild`)が無い環境では中止する(スキップを成功と数えない)。
  コマンドラインツールだけが選ばれている環境では `DEVELOPER_DIR` を Xcode に向ける。
- **積み上げた PR(base が main でない)**: ゲートは変えない。下位の PR が先にマージされてから、上位の PR の base を main に直して流す
  (`gh pr edit <番号> --base main`。base の変更はマージではない)。
- **所要時間**: XCUITest を含むため十数分〜数十分かかる。`run_in_background` で流す。シミュレータは `IOS_SIMULATOR` で指定でき、
  他のセッションが使っている機種を避ける。
- **却下した案**: (a) iOS のテストをゲートに入れず作成者の自己申告に任せる(スキップを成功と数える運用になる)。(b) `swift test` だけ流す
  (View・identifier・AX5 の不具合を検出できない。P6-25 では XCUITest だけが実装側の不具合 2 件を見つけた)。

