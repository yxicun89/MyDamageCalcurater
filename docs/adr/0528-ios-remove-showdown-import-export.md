# ADR-0528: iOS の Showdown 形式の書き出し・取り込みを廃止する(入口ごと外す) — G-03

- 状態: 採用(2026-10-11。ユーザー決定「Showdown 形式の取り込みは廃止する。入口ごと外す」)
- 関連: docs/usability-round3.md G-03、ADR-0506(日本語名の Showdown 風テキスト。本 ADR で廃止)、ADR-0502 と ADR-0501「P6-20」・ADR-0522(入口が折りたたみに移った経緯)

## 決定

1. 構築一覧の「Showdown 形式で取り込む」と構築編集画面の「Showdown 形式で書き出す」(どちらも折りたたみ)を外す。
   iOS のほかの画面(タイプバランス等)に Showdown の入口は元々無く、構築の 2 か所だけだった。
2. 取り込み・書き出しのコードは他で使われていないので**コードごと消す**:
   `ShowdownText.swift`(パーサ・シリアライザ・名前解決・文言)・`ShowdownTransfer.swift`(通信を伴う取り込み/書き出しと `TeamTextTransferViewModel`)・
   `TeamTextSections.swift` の折りたたみ(`TeamFold`)と取り込み/書き出しの部。`TeamLabels` の Showdown 文言(importFold・exportFold・importHelp・importExample ほか)。
3. 残すもの(他で使われている): `TeamTextControls.buttonLabel`(未保存の確認・削除の確認のボタン文言。`TeamTextControls.swift` に移した)。
   `TeamListViewModel.createTeam(members:)`(`TeamListViewModelRebuildTests` が使う〈作り直しの下地〉。コメントのみ直した)。
   「このアプリについて」の「Pokémon Showdown(MIT License)」はデータの照合元の表示で、取り込み機能とは無関係なので残す。
   `StatKey` の「Showdown 規約の並び」というコメントも並びの由来の説明なので残す。
4. 過去 ADR(0506・0502・0501「P6-20」・0522)の本文は書き換えない。0506 に「廃止」追記だけ足す。
5. 識別子の整理: `teamImportFold*`・`teamExportFold*`・`teamImportHelp`・`teamImportExample`・`importTextEditor`・`analyzeImportTextButton`・`importRejected*`・
   `confirmImportValidButton`・`cancelImportButton`・`exportTeamTextButton`・`exportedText`・`copyExportedTextButton`・`shareExportedTextLink`・`importedNotice` などは使わなくなった。
   README の該当節は廃止の 1 行に置き換えた。

## 影響

- API・契約・Web は変更なし(Web は別ブランチ)。保存済みの構築は影響を受けない。
- 構築の移し替え手段は、当面なし(必要になれば ADR で別方式を決める)。

## 追記: 製品コードから呼ばれなくなったが残すもの(critic 指摘)

`TeamEditViewModel.importMembers` と `Support/ConcurrencyProbeService.swift` は、入口の廃止で製品コードからの呼び出しが無くなった。
前者は `TeamEditViewModelImportTests`・`TeamEditViewModelSlotsTests` が枠の積み込み(6 体の境界・重複)の検査に使っており、
これらはテストの削除(絶対ルール6)を避けるため、構築の移し替えを別の形で再導入するときの土台として**残す**。
使い道が決まらないまま 1 年以上残る場合は別の変更で整理する。
