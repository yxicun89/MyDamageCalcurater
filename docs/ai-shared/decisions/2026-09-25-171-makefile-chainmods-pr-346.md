## 2026-09-25: 共有 Makefile の既存ターゲットの変更と chainMods のクランプ(データレーン。PR #346)
Decision: (1) ルート Makefile の既存ターゲット `test`(ゴールデンを含める)・`lint`(engine の vet に `-tags golden`・`-tags allspecies`)・`golden-generate`(先に npm ci)を変更した。COORDINATION.md は共有 Makefile を「自レーンのターゲットの追加」に限るが、issue #303・#77 が変更範囲として明示しているため。(2) engine の chainMods に @smogon/calc 0.12.0(`mechanics/util.js` の chainMods)と同じ下限・上限のクランプを入れた(最終補正 41..131072、威力 41..2097152、攻撃/防御 410..131072)。ADR-0002(ゴールデンの正は @smogon/calc 0.12.0)に沿って oracle と同じ挙動にそろえる変更で、既存のゴールデン全件一致を確認済み。
Reason: critic(PR #346)の軽微指摘。規約の外の変更と engine の計算の変更の根拠を記録に残す。
Impact: `make test` が約 3 秒長くなる(34.7s → 37.6s)。engine の既存の出力は変わらない(クランプの範囲外の値は既存のテストケースに無い)。
