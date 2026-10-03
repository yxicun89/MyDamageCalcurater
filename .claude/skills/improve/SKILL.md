---
name: improve
description: 動作確認で出た改善要望を受け取り、タスク化して docs/plan/improvements/<レーン>.md に追加し、ワークフローで実装する。例 /improve 結果の行をタップしたら詳細を開きたい
---
# /improve

要望: $ARGUMENTS

1. 要望を要件として解釈し、曖昧な点があれば **1つだけ** 質問する(明確なら質問しない)
2. requirements.md / design.md のどこを変えるか決め、該当箇所を更新する
3. `docs/plan/improvements/<レーン>.md`(自分のレーンのファイル。ADR-0172。`<レーン>` は `docs/ai-shared/state/` のファイル名と同じ: data(ダメージ計算・データ)・api・web・ios・type-balance・speed・judge・ops。無ければ同じ形で新規作成する)に `I-<レーン>-<連番>` としてタスクを追加する(例 `I-web-1`。連番はそのファイルの中で数える)
4. `/phase I-<レーン>-<連番>` と同じ手順で実装する
5. 変更点と、確認のために人間が触るべき操作を3行以内で報告する
