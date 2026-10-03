## Speed
Lane: 素早さ(素早さ比較サービス。`services/speed/`・`web/src/speed/`。どの AI が進めてもよい)
Active: なし(SP0〜SP5 すべて完了。次の要望待ち)
Branch: 次は main から feat/speed-<名前> を切る(作業ディレクトリ ~/MyDamageCalcurater-speed。SP5 は feat/speed-sp5 → PR #97 で main に統合)
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
Next: #263・#237 はタイプバランスレーン/APIレーンからの連絡待ち(連絡が来たら speed 側の overlay・scripts を対応)。
#105(Argo CD導入・digest固定の共有スクリプト化)は完了・追加対応不要。#108は データレーンからの連絡待ち(今は着手不要)。他は
balance-registry → pokecalc-registry への改名提案(タイプバランスレーンへ既定案で提示済み。DECISIONS.md 2026-09-23)かユーザーからの
新規要望待ち。
