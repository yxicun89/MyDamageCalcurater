# ADR-0520: ios-test-ui の所要時間(実測・2 並列・テスト単位の上限)と make ios-run

- 状態: 採用(2026-10-10)
- 日付: 2026-10-10
- 関連: ADR-0500 §7(iOS のターゲット)、ADR-0501 末尾「ios-test-ui の所要時間(ADR-0520)」、docs/usability-round2.md の F-14・F-15、
  `ios/scripts/run-xcode-tests.sh`・`ios/scripts/xcode-test-lock.sh`(排他ロック)

## 背景

F-14: 動作確認でシミュレータを動かす導線が `make ios-sim-run`(画面を指定してモックで起動し、スクリーンショットを撮る)しか無く、
実 API につなぐ・ただ起動して触る、がコマンド1つでできなかった。F-15: `ios-test-ui` が 20〜50 分(混むとそれ以上)かかる。

## 決定 1: `make ios-run`(F-14)

`ios/scripts/ios-run.sh`。シミュレータが止まっていれば boot して Simulator.app を開き(起動済みはそのまま使い、shutdown はしない)、
`xcodebuild build`(derivedData は `ios/build/DerivedData`。sim-run.sh と共有。`ios/build/` は .gitignore 済み)→ `simctl install` → `simctl launch`。
`POKECALC_API_BASE_URL` があればそれをビルドに渡す(接続先は Info.plist に焼かれるため。無ければ空 = モック)。`IOS_RUN_MOCK=1` で
`POKECALC_USE_MOCK=1` を起動環境(`SIMCTL_CHILD_`)に渡す。手順は docs/verify-m3.md §0。

## 決定 2: 実測(F-15)

2026-10-09 のゲートのログ(188 件 / 全成功)を集計した。

- 合計 4589 秒(約 76 分)。ログの開始〜終了の実時間とほぼ一致(テストは直列で、隙間は小さい)。1 件の平均 24 秒・中央値 17 秒・p90 49 秒・最長 133 秒・最短 5.5 秒。
- 起動の下限は約 5.5 秒(`openCalcScreen` を押して計算画面が出るまで)。188 件 × 約 5.6 秒 = 約 17 分(全体の約 23%)が起動の固定費。
- クラス別の合計: LargeTextLayoutUITests 852 秒(31 件)・JudgeScreenUITests 709 秒(17 件)・JudgeSpeedNotesUITests 617 秒(8 件)・FavoriteLoadUITests 242 秒・
  SpeedScreenUITests 237 秒。判定の 2 クラスだけで 1326 秒(29%)。
- 遅い上位: JudgeSpeedNotesUITests の麻痺の 2 件(133 秒・122 秒)、LargeTextLayout の判定 AX5 の 2 件(125 秒・94 秒)、他に判定画面の候補追加を伴う 70〜90 秒台。
  原因は 1 テスト内で候補を何件も足してから各行を検証する操作の量。固定の `sleep` は 2 か所のみ(最大 1 秒・0.1 秒刻みの回転待ち)で、短縮の対象になる無駄な待ちではない。
- 失敗・ハング風の主因は、複数セッションの同時実行による CPU の奪い合い(排他ロックで対処済み。ADR-0500 §7 / F-15 の既存対策)。

## 決定 3: 2 並列を既定にする(ui のみ)

`run-xcode-tests.sh` は label に `ui` を含むとき `-parallel-testing-enabled YES -maximum-parallel-testing-workers 2` を付ける。
`IOS_TEST_PARALLEL=0` で従来の直列に戻せる。

- 実測(6 クラス・36 件の部分集合): 直列 795 秒 → 2 並列 515・520・525 秒(3 回とも 36/36 成功。約 35% 短縮)。1 テストごとの秒数は変わらない。
  `Restarting after unexpected exit` は 0 回。
- 排他ロックとの両立: ロックは「この Mac で同時に 1 つの xcodebuild test」を保つもので変えない。並列はその 1 実行の内側でシミュレータのクローン
  (`Clone N of iPhone 18 Pro`)を作るだけで、`simctl shutdown all` は不要。クローンは実行後に消える。
- 件数の集計: 結果バンドルの summary はクローンをまたいで合算され、全件数と一致した(上の 36 件)。
- 採用条件(明確に速い・全件成功を 2 回以上再現)は部分集合で満たした。判定・AX5 を含む重い 3 クラス(56 件。最終のスクリプトで実行)は、
  直列の合計 2178 秒 → 2 並列 1511 秒(約 31% 短縮)で 56/56 成功。全件(188 件)での並列は未計測(予想は約 76 分 → 約 50 分)。全件で不安定になったら `IOS_TEST_PARALLEL=0` を既定に戻す。

## 決定 4: テスト単位の上限

`-test-timeouts-enabled YES -default-test-execution-time-allowance 300 -maximum-test-execution-time-allowance 600` を常に付ける
(`IOS_TEST_TIMEOUTS=0` で外せる)。実測の最長が 133 秒なので、既定はその 2 倍以上の 300 秒、上限は 600 秒。止まったテストは
その 1 件の失敗になり、数十分止まったまま気付けない状態を避ける。

## 不採用・見送り

- 待ち時間(`existenceTimeout` 等)の一律短縮: 起動の固定費は OS 側の待ち(idle 待ち)が大半で、タイムアウトを縮めても成功時の時間は変わらない。失敗の増加だけが残る。
- 3 並列以上: CPU を奪い合い、他レーンへの影響が増える。2 で止める。
- 判定の 2 クラス(最も遅い)の分割・テストの削減: テストを減らさない方針(絶対ルール6)。F-07 で判定の入口が既定で非表示になったため、判定のテストは環境変数で出して流す。
