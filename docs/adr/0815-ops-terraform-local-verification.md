# ADR-0815: Terraform は手元で確かめる(AWS は AWS Emulator、GCP は terraform test のモック)

- 状態: 採用(2026-11-13。ユーザー決定)
- 日付: 2026-11-13
- レーン: データ(到達経路・運用の持ち主。COORDINATION.md)
- 関連: docs/plan/cloud-design.md §5.1・§6・§6.1、ADR-0800(AI の権限ガード)、ADR-0814(クラウドの技術構成)

## 背景

cloud-design.md §5.1 の Terraform(`infra/terraform/`)をまだ作っていなかった。クラウドの apply・destroy は人間だけ(§6.1)で、
アカウントも未作成のため、書いた Terraform が動くかを確かめる手段が無かった。
2026-11-11 に AWS が公式の **AWS Emulator**(手元で動く AWS API の模造品。無料・本番利用不可)を公開した。
GCP には VPC・Compute Engine を扱える公式のエミュレータが無い(公式は Pub/Sub・Firestore・Spanner など一部だけ)。

## 決定

1. 最初のスタックとして **`10-network` だけ**を AWS・GCP の両方に作る(`infra/terraform/{aws,gcp}/10-network`。ユーザー決定)。
   VM・データ・bootstrap(予算・SCP・state バケット)は後のスタックで作る。backend は §8 Q7 が決まるまでローカル state。
   AWS は §5.1 の列挙に加えて、NAT 経由で外へ出るサブネットと経路(`enable_nat = true` のときだけ NAT・EIP・経路ができる)を置く。
   §5.4 の 2「NAT を一瞬だけ付けて請求の動きを見る」に要るため。VM 用サブネットは NAT を使わず、公開 IPv4 とインターネットゲートウェイで外へ出る
   (受信は SG で閉じる。公開 IPv4 は時間課金なので、§5.3 の棚卸しの対象)。
   GCP の `default_labels` は、VPC・サブネット・Cloud NAT がラベルに対応しないため、このスタックではどのリソースにも付かない(後の VM・バケットに付く)。
2. **GCP・AWS 共通: `terraform test` の `mock_provider` で計画を確かめる**(`make tf-test`)。クラウドにもエミュレータにも触れない。
   既定で NAT を作らない・受信を開けない・CIDR とリージョンの既定値、をテストで固定する。GCP の確認はこれだけ(ユーザー決定)。
3. **AWS だけ: AWS Emulator に apply → 差分 0 の plan → destroy を流す**(`make tf-aws-emulator`)。
   エミュレータは Linux パッケージ(署名検証して入れる)か、公式の Dockerfile の形(`infra/terraform/aws-emulator/Dockerfile`)で動かす。版は固定(1.0.343)。
   本物の AWS へ向かないよう、`AWS_*` の環境変数(鍵・プロファイル・ロール・サービス別エンドポイント)と `TF_VAR_*`・`TF_CLI_ARGS*` を全部外し、
   `AWS_ENDPOINT_URL` を `127.0.0.1`、CA をエミュレータの証明書にして、エミュレータが作る鍵だけを使う(`scripts/tf-aws-emulator_test.sh` で固定)。
   Docker のイメージは取得した RPM を SHA-256 で照合する。state はリポジトリの外の一時ディレクトリに置き、`*.tfvars` は持ち込まない。エミュレータへの apply は課金も本物の変更も無いので、§6.1 の「apply は人間だけ」の対象外とする。
4. エミュレータが再現しないリソースは、スタックの `emulator-known-gaps.txt` に理由付きで書き、それと、それを参照するリソースを除いて流す。
   除いたものは実行結果に「エミュレータで未検証」と出す。1.0.343 では `aws_security_group`(既定の送信ルールの削除で、ポートまで照合して NotFound を返す)。
5. **CI にはまだ入れない**(ユーザー決定)。`make tf-*` は `make test`・`make lint` に合成しない。入れるときは workflows の変更なので人間がマージする PR で行う。
   `scripts/tf-aws-emulator_test.sh`(STACK の検査と除外の計算。terraform もエミュレータも要らない)だけは `make test-scripts` に入れる。

## 影響

- 版は厳密に固定する: Terraform 1.17.2、hashicorp/aws 6.73.0、hashicorp/google 8.11.0。`.terraform.lock.hcl` は Linux・Mac のハッシュでコミットする(`make tf-lock`)。
- `.gitignore` に state・`*.tfvars`(`example.tfvars` を除く)・`.terraform/`・計画ファイルを足した。
- エミュレータは「API が受け付けるか」までで、本物の AWS と同じ結果の保証ではない(公式も忠実度をサービスごとの点数で公表している)。最終確認は人間の plan。
- エミュレータの版を上げたら、`emulator-known-gaps.txt` の行を消して通るか確かめる。
