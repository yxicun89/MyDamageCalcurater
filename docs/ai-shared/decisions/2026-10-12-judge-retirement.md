## 2026-10-12: 判定サービス(judge-svc)を廃止する(判定レーン。ADR-0715)
Decision: 連続技の計算画面と調整の目標で「抜いて倒せる」が確認できるので、judge-svc を廃止する。素早さの式・行動順は ADR-0179 で engine にある。Web は #753 で撤去済み。iOS が撤去した後、サービス・gateway の経路・k8s・GitOps・Makefile・archcheck の judge を消す。
Reason: ユーザー指示は「判定画面ではなく、ダメージ計算の拡張」。クライアントが無くなる。
Impact:
- iOS: 判定の撤去(Judge*、生成物 PokeCalcJudgeAPI、JudgeContractSync*、openapi-generator-config.judge.yaml、FeatureVisibility)。サービス側の削除より先。
- 実クラスタ: Argo CD の `pokecalc-judge` と Pod の削除はマージ後にユーザーの確認を取る。
