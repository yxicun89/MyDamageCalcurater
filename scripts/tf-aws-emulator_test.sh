#!/usr/bin/env bash
# scripts/tf-aws-emulator.sh と scripts/lib/tf-targets.py の自動テスト(ADR-0815)。
# `make test-scripts`(make test に含む)から流す。エミュレータ・terraform・ネットワークには触らない。
#
# 固定すること:
#   - STACK は infra/terraform/aws/ の下だけ受け付ける(GCP やリポジトリ外の構成をエミュレータに流さない)
#   - tf-targets.py は、除外したリソースと、それを参照するリソース(間接の参照も)を -target から外す
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
readonly ROOT

failures=0
ok() { echo "ok: $1"; }
ng() { echo "NG: $1" >&2; failures=$((failures + 1)); }

for stack in infra/terraform/gcp/10-network ../outside infra/terraform/aws/../gcp/10-network; do
  if STACK="$stack" "$ROOT/scripts/tf-aws-emulator.sh" >/dev/null 2>&1; then
    ng "STACK=$stack を受け付けた"
  else
    ok "STACK=$stack を拒否する"
  fi
done

plan_json='{
  "configuration": {"root_module": {"resources": [
    {"address": "aws_vpc.this", "expressions": {"cidr_block": {"references": ["var.vpc_cidr"]}}},
    {"address": "aws_security_group.vm", "expressions": {"vpc_id": {"references": ["aws_vpc.this.id", "aws_vpc.this"]}}},
    {"address": "aws_vpc_security_group_egress_rule.vm_all", "expressions": {"security_group_id": {"references": ["aws_security_group.vm.id", "aws_security_group.vm"]}}},
    {"address": "aws_instance.uses_rule", "depends_on": ["aws_vpc_security_group_egress_rule.vm_all"]},
    {"address": "aws_eip.nat", "expressions": {}}
  ]}},
  "resource_changes": [
    {"address": "aws_vpc.this"},
    {"address": "aws_security_group.vm"},
    {"address": "aws_vpc_security_group_egress_rule.vm_all"},
    {"address": "aws_instance.uses_rule"},
    {"address": "aws_eip.nat[0]"}
  ]
}'
got=$(echo "$plan_json" | python3 "$ROOT/scripts/lib/tf-targets.py" aws_security_group.vm | tr '\n' ' ')
want="aws_vpc.this aws_eip.nat[0] "
if [ "$got" = "$want" ]; then
  ok "tf-targets.py は除外したリソースと、それを直接・間接に参照するリソースを外す"
else
  ng "tf-targets.py の出力が違う: got=[$got] want=[$want]"
fi

got=$(echo "$plan_json" | python3 "$ROOT/scripts/lib/tf-targets.py" | wc -l | tr -d ' ')
if [ "$got" = "5" ]; then
  ok "tf-targets.py は除外が無ければ全リソースを出す"
else
  ng "tf-targets.py が除外なしで $got 件を出した(期待 5)"
fi

if [ "$failures" -gt 0 ]; then
  echo "tf-aws-emulator_test: $failures 件失敗" >&2
  exit 1
fi
echo "tf-aws-emulator_test: OK"
