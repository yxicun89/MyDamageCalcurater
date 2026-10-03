## 後続: 要件との対応(issue #286。M1〜M4 の後。担当レーン付き)
requirements.md の項目のうち、計画に無かったものをここに置く。着手の順・可否はユーザー判断(急ぎではない)。
- [ ] P5-3c お気に入り(手動ピン留め)の作成・削除・一覧 API と画面(requirements.md §2「あれば便利」。担当: API レーン→ Web・iOS。`favorites` の表・保持期間・全削除の件数は ADR-0209 で実装済みで、API・画面が未着手。ADR-0209 の「record にお気に入りの CRUD を足すときに検証する」を併せて行う)
- [ ] P6-20 iOS の Showdown 形式のインポート/エクスポート(requirements.md §2 は必須。P6-2 で後回しにしたまま。担当: iOS レーン。P5-4 の後。Web は P5-5・team-svc は P5-4)
- [ ] P8-1 ポケモン画像の配信(任意。M1 の後。requirements.md「ポケモン画像」: MinIO・gateway の画像パス・`manifest.json`・`make assets`・無ければタイプ色のエンブレム。担当: 運用(deploy・scripts)+ API + Web。gateway の予約パス `/assets/*` は未設定で常に 404 なので `/images/` に移す〈issue #286 所見1〉。`make assets` は実装まで終了コード 2 のスタブ)
- 公開時の名称・画像の差し替え構造(requirements.md「知財」): **後回し**。公開のタイミング(R-2-9・LICENSE・issue #328。ブロッカー節)と同時に決める。画像は P8-1 でキー(`{図鑑番号4桁}-{フォルム3桁}`)による差し替え構造になる
