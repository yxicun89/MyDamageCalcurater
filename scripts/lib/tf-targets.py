"""terraform show -json の計画から、除外するリソース(引数)とそれを参照するリソースを除いたアドレスを 1 行ずつ出す。

scripts/tf-aws-emulator.sh が、AWS Emulator の既知の差(emulator-known-gaps.txt)を除いて -target を作るのに使う。
使い方: terraform show -json plan.tfplan | python3 scripts/lib/tf-targets.py <除外するアドレス>...
除外するアドレスが構成に無ければ(書き間違い)終了コード 1 で止まる。モジュールは使わない前提(ルートのリソースだけを見る)。
"""

import json
import sys

NOT_RESOURCES = ("var.", "local.", "data.", "path.", "terraform.", "count.", "each.", "self.")


def resource_name(address):
    """aws_eip.nat[0] → aws_eip.nat(count・for_each の添字を外す)"""
    return address.split("[")[0]


def collect_references(node, refs):
    """式の木(入れ子のブロックは dict・list の中の dict)を辿り、references を全部集める"""
    if isinstance(node, dict):
        refs.update(node.get("references", []))
        for value in node.values():
            collect_references(value, refs)
    elif isinstance(node, list):
        for value in node:
            collect_references(value, refs)


def referenced_resources(resource):
    refs = set(resource.get("depends_on", []))
    for key in ("expressions", "count_expression", "for_each_expression"):
        collect_references(resource.get(key, {}), refs)
    return {".".join(ref.split(".")[:2]) for ref in refs if not ref.startswith(NOT_RESOURCES)}


def main():
    plan = json.load(sys.stdin)
    resources = plan.get("configuration", {}).get("root_module", {}).get("resources", [])
    refs = {r["address"]: referenced_resources(r) for r in resources}

    excluded = set(sys.argv[1:])
    unknown = sorted(excluded - refs.keys())
    if unknown:
        print("tf-targets: 構成に無いリソースを除外しようとした: " + " ".join(unknown), file=sys.stderr)
        sys.exit(1)

    changed = True
    while changed:
        changed = False
        for address, used in refs.items():
            if address not in excluded and used & excluded:
                excluded.add(address)
                changed = True

    for change in plan.get("resource_changes", []):
        name = resource_name(change["address"])
        if name in refs and name not in excluded:
            print(change["address"])


if __name__ == "__main__":
    main()
