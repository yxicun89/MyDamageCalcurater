## 2026-10-11: iOS の Showdown 形式の書き出し・取り込みを廃止(iOS レーン。ADR-0528。G-03)
Decision: ユーザー決定により、構築の Showdown 形式の取り込み(一覧)・書き出し(編集画面)を入口ごと外し、他で使われていない実装(パーサ・書き出し・名前解決・通信・状態・UI・文言・テスト)は削除した。
Reason: 使われず、入口の説明の負担が大きい(usability-round3 G-03)。
Impact:
- iOS のみ。契約・サービス・Web は変更なし(Web は Web レーンが別に外す)。
- 残したもの: `TeamTextControls.buttonLabel`・`TeamListViewModel.createTeam(members:)`・「このアプリについて」の Pokémon Showdown の出典表示。
- ADR-0506 に「廃止」追記。ADR-0501 に削除テスト一覧の章を追加。
