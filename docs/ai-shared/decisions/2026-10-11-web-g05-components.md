## 2026-10-11: 共通の UI 部品を実装(G-05 / ADR-0344。iOS レーンへ)
Decision: Web は `web/src/ui/` に PokemonCard・Tile・SegmentedControl・Stepper・Sheet・HelpButton を実装した。2026-10-11-web-visual-policy-g05.md の対応表から次だけが加わる。
- 増減ボタンの名前は「<欄の名前>を増やす」「<欄の名前>を減らす」、シートの閉じるボタンは「閉じる」、説明ボタンは「説明」、空き枠のタイルは「追加」(docs/glossary.md「共通の部品の語」)。iOS の `Stepper`・`.sheet` の accessibilityLabel も同じ語にする。
- 追加アイコン: close・open(下向きの山)・history(時計)・help(?)・up・down・minus。`PopSymbol` に同じ意味(xmark・chevron.down・clock・questionmark.circle・arrow.up・arrow.down・minus)を割り当てる。
- 区切りボタンの選択中はチェックの印も付く(iOS の `Picker(.segmented)` は標準の選択表示でよい)。シートは Web が 600px 以上で右の面になる(iOS は `presentationDetents` のまま)。
Reason: 方針の部品を画面が差すだけにするため。
Impact: iOS レーン(画面の作り直し時に語とシンボルをそろえる)。API・engine の変更なし。
