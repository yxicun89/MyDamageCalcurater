## 2026-10-04: 調整のメガ固定と「目標から振り方を決める」の段階 A(Web レーン。ADR-0331)
Decision: 調整画面は、自分がメガ種族なら持ち物をストーンに固定する(disabled・理由の文・aria-describedby。非メガに変えたら未選択に戻す)。要求(indices・各モード・goals)の自分の `itemId` にストーンを送り、相手がメガなら相手にも送る。ストーンを引けないときは送らない。「目標から振り方を決める」モードは画面とクライアントを入れたが、calc-svc の `/api/calc/adjust/goals` が実装される(段階 B)まで `ADJUST_GOALS_ENABLED` = false で利用者に見せない。
Reason: ユーザー要望 F-10・F-11(docs/usability-round2.md)。iOS の調整(ADR-0509)は固定済みで、Web をそろえる。目標の最小の振り方は既存の4操作では作れないため新しい操作を足した(ADR-0331 §2)。
Impact:
- ダメージ計算レーン: 段階 B(engine の複数目標探索・calc-svc の実装)を行い、入ったら `ADJUST_GOALS_ENABLED` を true にする。
- iOS: 目標モードは段階 B の後に同じ契約(`adjustGoals`)で追従する。相手のストーンを要求に載せる点だけ確認する。
