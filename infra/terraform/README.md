# infra/terraform

クラウドの Terraform(cloud-design.md §5.1)。いまは `10-network`(AWS・GCP)だけ。
手元での確かめ方は ADR-0815。本物のクラウドへの `apply`・`destroy` は人間だけが行う(cloud-design.md §6.1)。

## 手元で確かめる

前提: Terraform 1.17.2(`terraform version`)。

```
cd "$(git rev-parse --show-toplevel)"
make tf-fmt-check tf-validate tf-test
```

確認点: スタックごとに `Success! The configuration is valid.` と `Success! N passed, 0 failed.` が出る。

## AWS Emulator に流す(AWS だけ)

Mac(Docker Desktop が動いていること。初回はイメージを作る):

```
cd "$(git rev-parse --show-toplevel)"
make tf-aws-emulator
```

Linux(Docker を使わずパッケージを入れる場合。x86_64):

```
cd "$(git rev-parse --show-toplevel)"
curl -fsSLO https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/aws-emulator_1.0.343-1_amd64.deb
curl -fsSLO https://downloads.aws.com/aws-emulator/packages/aws-emulator/LTS/aws-emulator_1.0.343-1_amd64.deb.sig
curl -fsSLo aws-publishing.asc https://downloads.aws.com/keys/gpg/current.asc
gpg --import aws-publishing.asc
gpg --fingerprint ssm-publishing@amazon.com
gpg --verify aws-emulator_1.0.343-1_amd64.deb.sig aws-emulator_1.0.343-1_amd64.deb
sudo apt-get install -y ./aws-emulator_1.0.343-1_amd64.deb
rm aws-emulator_1.0.343-1_amd64.deb aws-emulator_1.0.343-1_amd64.deb.sig aws-publishing.asc
make tf-aws-emulator
```

確認点: 指紋が `98BC F336 A3BE 53B7 34E9 67C5 2E5B F49B D940 F624` で `Good signature`、最後に `tf-aws-emulator: OK(infra/terraform/aws/10-network)` が出る。
「エミュレータで未検証」の行は、エミュレータが再現しないリソース(`emulator-known-gaps.txt`)。

## provider の版を上げたとき

```
cd "$(git rev-parse --show-toplevel)"
make tf-lock tf-validate tf-test
```

確認点: 各スタックの `.terraform.lock.hcl` が更新され、テストが通る。
