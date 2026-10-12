# ADR-0715: 判定サービス(judge-svc)を廃止する(連続技 CB-7)

- 状態: 採用(2026-10-12。判定レーンの判断。ユーザー決定: 判定の画面はダメージ計算の拡張に置き換える)
- 日付: 2026-10-12
- 関連: ADR-0179(素早さ・行動順の engine への移動)、ADR-0700〜0714(判定)、ADR-0709(判定の GitOps)、ADR-0416(gateway 経由)、docs/plan/combo-calc.md §4-2・CB-7

## 背景

ユーザーの指示は「判定の画面ではなく、ダメージ計算の拡張」だった。連続技(CB-1〜CB-6)が main に入り、
素早さを抜く・倒す・耐えるの確認は計算画面と調整の目標で足りる。判定の素早さの式と行動順は、CB-2(ADR-0179)で engine に移っている。
判定の契約(`outspeed-and-ko`)を使うクライアントは、Web(#753 で撤去)と iOS(撤去は iOS レーン)だけだった。

## 決定

1. judge-svc を廃止する。`services/judge/`、gateway の `/api/judge/*` の経路(`GATEWAY_JUDGE_URL`)、k8s の base・networkpolicy・観測(ServiceMonitor)、
   GitOps の overlay と Argo CD の Application、Makefile の judge ターゲット、go.work の項目、`tools/archcheck` の unmigrated の judge の行を消す。
2. 順序: Web の撤去(#753)→ iOS の撤去(iOS レーン。JudgeContractSync が契約ファイルを読むため先)→ サービス側(本 ADR)。
3. `/api/judge/*` は、廃止後は gateway の未知のパスと同じ 404 になる。
4. 過去の ADR(0700〜0714)の本文は変えない。判定の画面を戻す場合は、engine の素早さ・行動順(ADR-0179)の上に作り直す。
5. 実クラスタの Argo CD Application `pokecalc-judge` と judge の Pod の削除は、マージ後にユーザーの確認を取って行う(クラスタ操作)。

## 影響

- 他レーン: iOS は判定の撤去(別 PR)。Web は撤去済み。API・engine は不変。
- CI のワークフローを変える場合は人間がマージする(今回は judge のコメントのみで、ロジック変更なし)。
