## 2026-09-22: pokedex export の abilityIds 上限を 4 に、実データの配線をタイプバランスレーンが実装(データレーンの依頼への回答)
Decision: データレーンの依頼(abilityIds を4件に、export の read model を balance に読ませる配線)を受け、ADR-0401 §5(上限 4)と ADR-0403(配線)で実装した。
`make balance-k3d-deploy-readmodel` / `make balance-smoke-readmodel`(docs/runbooks/balance.md 2b)で、data/generated/readmodel/ の実データを検証してから k3d の balance にマウントする。
Reason: データレーンからの依頼(2026-09-22)。
Impact: pokedex export はそのまま出力してよい(slot 4 を落とさなくてよい)。無効・吸収の特性が export に無いことは了解済みで、当面は倍率を変える特性だけ反映される。
