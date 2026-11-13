"""terraform show -json の計画から、除外するリソース(引数)とそれを参照するリソースを除いたアドレスを 1 行ずつ出す。

scripts/tf-aws-emulator.sh が、AWS Emulator の既知の差(emulator-known-gaps.txt)を除いて -target を作るのに使う。
使い方: terraform show -json plan.tfplan | python3 scripts/lib/tf-targets.py <除外するアドレス>...
"""

import json
import sys


def resource_name(address):
    """aws_eip.nat[0] → aws_eip.nat(count・for_each の添字を外す)"""
    return address.split("[")[0]


def referenced_resources(resource):
    refs = set(resource.get("depends_on", []))
    for expr in resource.get("expressions", {}).values():
        if isinstance(expr, dict):
            refs.update(expr.get("references", []))
    return {
        ".".join(ref.split(".")[:2])
        for ref in refs
        if not ref.startswith(("var.", "local.", "data.", "path.", "terraform."))
    }


def main():
    plan = json.load(sys.stdin)
    resources = plan.get("configuration", {}).get("root_module", {}).get("resources", [])
    refs = {r["address"]: referenced_resources(r) for r in resources}

    excluded = set(sys.argv[1:])
    changed = True
    while changed:
        changed = False
        for address, used in refs.items():
            if address not in excluded and used & excluded:
                excluded.add(address)
                changed = True

    for change in plan.get("resource_changes", []):
        if resource_name(change["address"]) not in excluded:
            print(change["address"])


if __name__ == "__main__":
    main()
