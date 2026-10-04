## 2026-10-04: メガストーンを取得元から判定し、メガのフォーム名の判定を広げた(データレーン → Web・iOS レーン。issue #607・ADR-0140)
Decision: メガ種族はフォーム名に `Mega` を含むもの(`Mega`・`Mega-X`・`M-Mega`・`F-Mega` など)とする。持ち物の `isMegaStone` は
Showdown の `megaStone` がある、または取り込んだメガ種族が要求する持ち物のとき真(食い違いは警告 `item-mega-stone-mismatch`)。
日本語名が上流に無いメガストーン(Champions で新しく出た39件)は推測で作らず、上書き設定で補う。
Reason: ユーザー要望(メガ種族を選ぶと持ち物に正式名称が出る)。ニャオニクスのメガなどがメガとして扱われていなかった。
Impact:
- **Web・iOS へ**: 持ち物の nameJa に日本語が無いメガストーンは「{基本種名}のメガストーン」と表示する(表示は各レーン)。API 契約は変わらない
- 既存の取得物は megaStone キーが無いので取り直しが要る(`make import-fetch`。CronJob は自動)
- 反映: deploy-latest → `make import-k8s` → `make master-release`
