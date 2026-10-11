## 2026-10-11: Showdown 形式の取り込み・書き出しを廃止する(Web。G-03 / ADR-0342。iOS レーンへ)
Decision: ユーザー決定により Showdown 形式のインポート/エクスポートを廃止。Web は構築画面の入口(一覧の取り込み・編集画面の書き出し)と、変換部・UI・文言・専用テストを削除した。ほかの Web 画面に入口は無かった。
「このアプリについて」の出典「Pokémon Showdown(MIT License)」は取得元の表記なので残す(iOS の AboutText も同じ)。API・team-svc・保存済みの構築データは変えない。
Reason: ユーザー要望(docs/usability-round3.md G-03。使いにくいので入口ごと外す)。
Impact: iOS レーンは構築画面の取り込み・書き出しの入口(ADR-0506 の日本語名 Showdown 風テキスト)と、他で使われていなければその変換部・文言・テストを外す。docs/plan/m3.md P6-20 は履歴として残し、廃止の注記は Web 側で入れた。
