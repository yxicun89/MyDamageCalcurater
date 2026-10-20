## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: なし(SP0〜SP5 すべて完了。次の要望待ち)
Branch: 次は main から feat/speed-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-speed。SP5 は feat/speed-sp5 → PR #97 で main に統合)
Status(追記): 2026-10-21 ARCH S9 完了(データレーンが代行。ブランチ feat/arch-s9-speed)。`internal/speed` → `internal/domain`、`master`・`httpapi`・`httpguard`・`httpmetrics` → `internal/adapters/`、`internal/app` は doc.go だけ。振る舞いは不変・archcheck の未移行の一覧から speed を外した(ADR-0811)
Status: SP0〜SP5 すべて完了・main に統合(PR #32・#36・#52・#83・#86・#93・#97)。SP4 の実データ確認はユーザーが2026-09-24 に実施:
`mysql` Service はクラスタ内部の DNS 名で Mac からは解決できないため、`kubectl -n pokecalc port-forward svc/mysql 3306:3306` を張り、
DSN のホストを `127.0.0.1` に付け替えて `make pokedex-export`(348 pokemon)→ `make speed-k3d-deploy-readmodel` →
`make speed-smoke-readmodel` を実行(初回はロールアウト直後で 504、再実行で `speed readmodel smoke: pokemon=0003-000 list=200 table=200`)。
SP5 の実際の Argo CD への適用(`speed-argocd-app`・`speed-registry-push`・sync)は未実施のまま(ADR-0605 §4。共有クラスタへの変更のため
人間の確認のもとで、必要になったときに)
Status(追記): DOC-arch(docs/architecture.mdを全レーンの現行構成に合わせて更新。ユーザー依頼)を素早さレーンが担当・完了(PR #168・#194)。
judge-svc を全体図・コンポーネント表に追加(先に判定レーンの抜けを見つけて#168で対応)、record-svc・team-svc(M2。計画中。
services/record・services/teamはまだ.gitkeepのみ)をTiDB・NATS JetStreamとあわせて追加、Kustomize overlay(local/cloud)の
節を新設。gateway・pokedex・calc・balance・speed・judge・データの流れ・WASMはコードを確認し既に現行と一致(変更なし)。
Status(追記): 2026-09-25、全体レビュー issue の割り当てミス(#71・#74・#76・#77 は素早さ担当ではなかった)を指摘し、
データレーンへ差し戻し済み。素早さが実際に担当に入る open issue を洗い出し: #263(タイプバランス主・素早さ・運用。Argo CD
Applicationのproject: default・初期admin Secret残存・GitOpsスクリプト5本の重複)・#237(タイプバランス・素早さ。needs-decision。
GitOps overlayがread modelを持たずbalance/speedの業務APIが全て503。既知の制約はADR-0605 §2aに記載済み)・#236(タイプバランス・
素早さ・判定・API連携。端末ID/セッションIDの検証とエラーコードがgatewayと3サービスで不一致)・#108(既知・データレーン主担当)。
タイプバランスレーンと分担を確認済み: #263はタイプバランスレーンが主担当(speed側の差分は連絡が来たら対応)、#237は既定案
(initContainerでpokedex exportを起動時に実行)でユーザー確認中(タイプバランスレーンが担当)、#236は共通パッケージの置き場所を
タイプバランスレーンがAPIレーンと相談中。#237の実装には「pokedex-svcのserverイメージをbalance-registryへdigest固定でpush」という
データレーンへの新しい依頼が発生することをタイプバランスレーンに共有済み。
Status(追記): 2026-09-25、#236のspeed側を完了(ADR-0606。PR作成中)。gatewayのcheckAPIHeaders/isCanonicalUUIDを
`services/speed/internal/httpapi/requestctx.go`に複製(httpmetricsと同じ前例。共通パッケージ新設なし、APIレーン合意済み)。
X-Device-Id/X-Session-Idの検証を正準形UUIDに強化し、エラーcodeを`invalid_request`から`missing_header`/`invalid_header`
へ分離(契約の破壊的変更)。openapi.yaml 0.4.0・web/src/speed/speed.gen.tsを再生成・critic PASS。balance・judgeは各自対応。
Status(追記): 2026-10-02 issue 307(素早さ画面の範囲外入力)を解消(critic PASS〈1回目〉)。カスタムの SP(0〜32)・ランク(-6〜+6)と
実数値(1以上の整数)の範囲外は、送信前に日本語の role=alert・aria-invalid で止めて API を呼ばない。実数値の上限は契約に無いので
画面では検査せず、API の 400 を `errorByCode` で日本語にする。空欄はカスタム SP・ランクは 0 とみなし、実数値は未入力で呼ばない。
既知の積み残し: 数値欄で「-」を打つと値が空になり 0 に戻るため負数をキー入力しづらい(従来からの挙動。直すなら欄の state を文字列で持つ)・
検証規則が JudgeScreen の validationMessage と二重管理(将来の共通化候補)。
Status(追記): 2026-10-03 素早さの SP6(追い風・まひ・トリックルーム。ユーザー決定。特性は対象外)を完了(ADR-0607。PR #485。critic〈Opus〉PASS)。
追い風 ×2 は(追い風 8192・スカーフ 6144 を4096基準で連結)→1回だけ五捨五超入(判定サービスと同じ式)、まひ ×0.5 は連結の後に整数 floor(v*50/100)
(@smogon/calc 0.12.0 Champions 世代で確認)、トリックルームは表の段の順だけを反転(同速は1段・段内は反転しない)。契約 0.5.0 は追加のみ・後方互換。
素早さの 999 上限は入れない(実機で 999 超。ユーザー回答)。speed-design.md に「9. テスト」節(PR #456)。
運用: PR #484(ADR-0801)で speed に共通の httpguard(同時実行上限で 503 overloaded+Retry-After・締め切り)、PR #487(ADR-0802)で未知ルート/405 の
404 not_found JSON と panic の 500 internal_error JSON、PR #520(ADR-0804)で preStop と停止処理を追加。
Status(追記): 2026-10-11 G-04 / I-speed-3(Web)完了。「自分の周り」パネル(ADR-0609。buildNeighborhood と SpeedNeighborhoodPanel。`.speed-self` の結果の下・表のスクロール領域の外)を実装。iOS は後続 PR。
Status(追記): 2026-10-11 G-04 / I-speed-3-ios 完了。「自分の周り」パネル(ADR-0527。SpeedNeighborhoodBuilder.build・SpeedViewModel.neighborhood・SpeedNeighborhoodSection)を実装し、自分のカード内の SpeedResultView の下に差し込んだ(表の LazyVStack の外)。
Next: #263・#292・#284 は他レーンが main 統合済みで、素早さ側の追加対応は無い。#237・#235 は人間の判断待ち。#108 は balance・speed 本体の loader が
dataVersion を保持・表示する変更が残る(ADR-0135。各レーンの持ち物。今は配備時の Deployment 注釈で代用)。#523(calc の shutdownTimeout)は API レーン向け。
実地確認が未実施: #299(Docker 負荷試験・k3d の Traefik HelmChartConfig)と、k3d 上の版一致確認(make check-master-version)。
既知の積み残し: 素早さ画面の数値欄で「-」を打つと 0 に戻り負数をキー入力しづらい(従来から)。
(旧)Next: #263・#237 はタイプバランスレーン/APIレーンからの連絡待ち(連絡が来たら speed 側の overlay・scripts を対応)。
#105(Argo CD導入・digest固定の共有スクリプト化)は完了・追加対応不要。#108は データレーンからの連絡待ち(今は着手不要)。他は
balance-registry → pokecalc-registry への改名提案(タイプバランスレーンへ既定案で提示済み。DECISIONS.md 2026-09-23)かユーザーからの
新規要望待ち。

Status(追記): 2026-10-04 F-06 の Web 分(I-speed-1。ADR-0608)を完了。素早さ表の左の一覧を仮想スクロールにし(自前の windowing・依存追加なし)、右の位置マーカーを左の該当行と同じ高さに揃えた。段の高さは同速の行数で決まる(1行 24px 固定・折り返さない)。iOS 分は未(別 PR)。
Next: F-12(ビジュアルの基盤)が入ったら、素早さ画面の見た目を揃える。そのとき speedWindow.ts の高さの定数(ENTRY_HEIGHT など)と行の高さを保つこと(ADR-0608 §結果)。

Status(追記): 2026-10-04 iOS の素早さ表の遅延描画と自分の位置の表示(F-06 の iOS 分。I-speed-2。ADR-0517)を実装(PR 参照)。

Status(追記): 2026-10-04 main の `make ios-test` が #628 以降赤だった件を直した(AX5 の素早さ表のテスト。原因はテストの操作のみ。実装の欠落なし。ADR-0517 追記)。

Status(追記): 2026-10-09 #108 の素早さ分を完了。speed 本体の loader・起動ログ・`/healthz` の dataVersion は ADR-0138 で実装済みだったため、残りだった `scripts/check-master-version.sh` の speed 判定を、Deployment 注釈から `/healthz`(サービスプロキシ経由。プロセスが読んだ版)に切り替えた(ADR-0809)。取れなければ注釈へ戻らず STALE。balance は注釈のまま(タイプバランスレーン)。
Next: k3d 上の実地確認(`make check-master-version`)は未実施。balance の healthz 参照への切り替えはタイプバランスレーンで(ADR-0809 §影響)。
Status(追記): 2026-10-12 I-speed-4a(バックエンド)完了。`internal/speed/matchup.go`(MovesFirstLines・Matchup・Reach。素早さは Speed〈engine 経由〉、行動順は engine.CompareSpeed)と `internal/httpapi/matchup.go`(POST /matchup・/reach。{guard, requireRequestContext})を実装。契約 0.7.0 は spec 側で追加済み。Web(I-speed-4b)・iOS(I-speed-4c)は別 PR。ADR-0610 の §6 Q1〜Q7 は推奨案のまま実装(ユーザー確認待ち)。
Status(追記): 2026-10-12 I-speed-4c(iOS の画面)の仕様とテストを先に書いた(ブランチ feat/speed-ios-matchup。コミットのみ・push 前。ビルドは通り、新しいテストが失敗する状態)。
設計は ADR-0610 追記「iOS の画面」。PokeCalcKit に matchup・reach のドメイン型・`SpeedService` の 2 操作・`SpeedMatchupViewModel`・`SpeedMatchupLabels`・`SpeedMatchupDisplayBuilder` のスタブ、UI テスト 4 本(`SpeedMatchupUITests` 3 本・`SpeedMatchupLargeTextUITests` 1 本)。
旧 `SpeedViewModel` と表・周辺パネルの部品・テストは、画面の置き換えと一緒に implementer が整理する(ADR-0610 追記 §5・§6)。ランクのメーターは iOS に部品が無く、今回は `SpeedStepper` を使う。
Next: implementer が docs/plan/improvements/speed.md の I-speed-4c の順(API クライアント → ラベル・整形 → ViewModel → モック → View → 旧 UI テストの整理)で実装する。Web(I-speed-4b)の語と食い違ったら ADR-0610 追記に差を書く。
Status(追記): 2026-10-12 I-speed-4c(iOS の画面)を実装(ブランチ feat/speed-ios-matchup。コミットのみ・push 前)。`APISpeedService` / `MockSpeedService` の matchup・reach、`SpeedMatchupLabels`・`SpeedMatchupDisplayBuilder`・`SpeedMatchupViewModel` を実装し、素早さ画面を新構成(入力 = 自分・相手・想定のチップ・今の振り・場、出力 = 想定ごとのカード + 「これ以上」、連続技へのリンクは `comboAvailable` が true の間だけ)に作り直した。表・周辺パネル・旧 `SpeedViewModel` と専用のコード・テストは削除(ADR-0610 追記 §5)。Web との文言の差は ADR-0610 追記 §8。
Next: I-speed-4b(Web)とあわせて PR を出す。連続技(CB-8)が計算画面に入ったら `SpeedScreenView` の `onOpenCombo` を計算画面へつなぐ。ランクのメーター化は iOS 全体のランク入力をそろえるときの別タスク。
Status(追記): 2026-10-12 I-speed-4b(Web)の仕様とテストを先に書いた(spec-writer。ブランチ feat/speed-web-matchup)。ADR-0610 に §7「Web 画面の具体」を追記し、ADR-0608・0609 に「ADR-0610 で置き換え」を追記。失敗するテスト: `web/src/speed/SpeedScreen.test.tsx`(W1〜W12)・`SpeedScreen.images.test.tsx`・`speedClient.matchup.test.ts`、`web/e2e/speed.spec.ts` を書き換え。旧画面専用のコード・テスト(speedWindow・speedNeighborhood・SpeedNeighborhoodPanel・旧 SpeedScreen*.test)は削除。`SpeedScreen.tsx` と `speedClient.ts` の `matchup` / `reach` はスタブ。連続技の Web 画面が画面レジストリ(ADR-0323)に無いので、連続技へのリンクは `onOpenCombo` を渡されたときだけ出す(今は渡さない)。
Next: implementer が上のテストを通す(I-speed-4b)。その後 iOS(I-speed-4c)。ADR-0610 §6 Q1〜Q7 は推奨案のまま(ユーザー確認待ち)。
Status(追記): 2026-10-12 I-speed-4b(Web)の実装を完了(feat/speed-web-matchup)。`SpeedScreen.tsx` を新構成(自分・相手・場 → 想定ごとのカード → 「これ以上」)に書き直し、`speedClient.ts` の matchup / reach を postJson で実装、`SpeedScreen.css` を作り直した。App.routing.test の「/table を呼ぶ」期待は表が画面から外れたので「/pokemon のみ・table は呼ばない」に付け替え。連続技へのリンクは `onOpenCombo` が渡されたときだけ(speed.screen.tsx は渡さない)。Next: I-speed-4c(iOS)。e2e の「キーボード」テストは macOS の chromium で select の ArrowDown がポップアップを開くため赤(Linux CI では値が変わる想定)。
