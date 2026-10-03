## 2026-09-24: getMovesByIdsのids上限64件について実データを確認(API レーン → Web レーンへ訂正)
Decision: PR #196マージ時点でDocker/k3dクラスタが停止しており検証できなかった「1種族のlearnsetが64件を
超えるか」を、クラスタ復旧後に実クラスタ(k3d-pokecalc)で確認した。
```
SELECT COUNT(*) FROM (
  SELECT l.species_key, COUNT(*) cnt FROM learnsets l
  JOIN regulation_moves rm ON rm.move_id = l.move_id
  WHERE rm.regulation_id = (SELECT id FROM regulations WHERE is_default = 1)
  GROUP BY l.species_key HAVING cnt > 64
) t;
```
結果: 既定のレギュレーション(M-C)で**349種族中151種族(43%)が64件を超え、最大106件**(図鑑番号0475。
メガ進化フォームも同数)。ADR-0304 §3が当初書いていた「20〜30件」という目算は大幅に外れていた。
**64件を超えるlearnsetはまれな例外ではなく、ごく普通に起こる**。ADR-0105 §3・ADR-0304 §3を実測値で更新した。
Reason: 「1回で必ず収まる前提は置かない」という設計(PR #196で既に反映済み)自体は正しかったが、
「まれなケースの保険」ではなく「日常的に発生する分割呼び出し」であることをWebレーンに正確に伝える必要がある。
Impact: **Webレーンへ訂正**: `getMovesByIds`の分割呼び出しは例外処理ではなく主経路として実装すること
(151/349種族=43%で必要になる)。前回の連絡(2026-09-24早め)で「64件超の実データ確認はまだ」と伝えていたが、
今回確認が取れたので更新する。設計・APIの変更は無し(64件という上限値自体は据え置き。ADR-0105・ADR-0304参照)。
