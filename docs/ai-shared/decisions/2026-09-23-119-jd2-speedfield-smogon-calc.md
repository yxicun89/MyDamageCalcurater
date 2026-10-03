## 2026-09-23: 判定 JD2 の設計確定(場の効果 `speedField`)。素早さ補正の連結と丸めを @smogon/calc で確認(判定レーン)
Decision: ADR-0702 で JD2 を確定した。`POST /api/judge/v1/outspeed-and-ko` に省略可の `speedField`
(`trickRoom`・`attackerTailwind`・`defenderTailwind`。すべて既定 false)を足す。calc-svc へ転送する `field` とは別の欄にし、`speedField` は calc-svc に送らない。
追い風は実数値を ×2 し、トリックルームは実数値を変えず `outspeeds`(自分が先に動くか)の比較の向きだけを反転する(`speedTie` は反転しない)。
**素早さ補正は 4096 基準で 1 つに連結してから 1 回だけ五捨五超入する**(補正ごとに丸めない)。追い風 8192・こだわりスカーフ 6144。
Reason: 丸めの規約(CLAUDE.md「4096基準の固定小数と五捨五超入」)に関わるため推測せず、ADR-0002 が固定した @smogon/calc 0.12.0 の
`dist/mechanics/util.js` の `getFinalSpeed` を実際に読んで確認した(Champions 世代も `computeFinalStats` 経由で同じ関数を通る)。
原典は `speedMods` に追い風 8192・スカーフ 6144 を積み、`chainMods`(1 ステップは切り上げ寄り)で 1 つにまとめてから `pokeRound`(五捨五超入)を 1 回だけ掛ける。
実数値 91 にスカーフと追い風が両方乗ると 273 になり、補正ごとに丸める実装(136 → ×2)だと 272 で 1 ずれる。
Impact: judge は engine の非公開 `chainMods` / `pokeRound` を呼べないため、同じ式を `internal/judge` に名前付き定数で持つ(ADR-0600 §3・ADR-0701 §2 と同じ扱い)。
`CompareSpeed` は引数を 1 つ(`SpeedField`)足す形に変えた(既存テストの期待値は変えていない)。麻痺(`status`)は連結の後に別枠で掛かり、ダメージ側にも効くため JD2 に含めない。
実装(implementer)は未着手で、`make judge-test` は失敗したままにしてある。
