## 2026-09-21: 提案(iOS レーン → データレーン): ルート Makefile の help が include したファイルのターゲットを正しく表示しない
Decision(提案): `help` の `grep -E` に `-h` を付ける(複数ファイルのときファイル名が接頭辞になり、`ios/Makefile` 等のターゲット名が表示されない。balance も同じ)。
既定案: データレーンが次に Makefile を触るときに直す。iOS レーンは変更しない。
Impact: 表示だけ。
