# 判断待ちの一覧(PENDING)

人間の判断待ち・既定案で進行中(ユーザー未確認)・未回答の提案を **1 行ずつ**置く。ユーザーはここを見れば、自分の判断を待つ項目が全部分かる。
(2026-10-03 新設。issue #229。ユーザー決定「既定案で OK」)

- 新しい提案・判断待ちを `decisions/` に書くときは、**ここにも 1 行足す**(COORDINATION.md)。
- 回答済み・対応済みの行は**消す**(経過は `decisions/` と issue に残る)。
- 並びは種別ごと、起票日の古い順。issue があるものは issue を正とし、ここは索引にとどめる。
- 深夜(23:00〜8:00 日本時間)は質問せず、ここへ既定案つきで書いて作業を続ける(COORDINATION.md「人間への質問」)。

## 人間の判断待ち

| 内容 | 場所 | 起票 | 宛先レーン |
|---|---|---|---|
| クラウド選定(AWS/GCP)・月額上限(設計案と質問 Q1〜Q10: docs/plan/cloud-design.md §8) | issue #149 | 2026-09-23 | 運用(データ) |
| クラウド公開前のアクセス境界と認証方針 | issue #148 | 2026-09-23 | 全レーン |
| リポジトリの公開状態(公開するか。LICENSE・クリーンコピー・第三者データを含まない確認。R-2-9) | docs/plan/blockers.md・issue #328 | 2026-09-21 | データ(ユーザー) |
| Argo CD のリポジトリ認証(PAT 等の登録。非公開のままなら必要。balance・speed・judge の Application が取得できる状態) | state/type-balance.md Next ③・ADR-0605 | 2026-09-24 | データ(ユーザー) |
| 新しいクラスタでの `docs/verify-m1.md` §3 の通し実行(Secret・DB・マスタ投入・NetworkPolicy。人間の確認つき) | docs/verify.md §1-1・state/data.md | 2026-10-03 | データ(ユーザー) |
| P4-5 の Safari 実機確認(Chrome は確認済み。作業は止めない) | docs/plan/blockers.md | 2026-09-22 | Web(ユーザー) |
| 失効ジョブ(record-expire・team-expire)を k3d の実データへ初めて向ける承認 | docs/plan/blockers.md・ADR-0209・ADR-0220 | 2026-09-26 | API(ユーザー) |
| 共有クラスタへの apply(NetworkPolicy・Grafana 管理者 Secret・pokedex の実 digest 確定 → overlay の PR → Sync) | state/type-balance.md Next・issue #237 | 2026-09-24 | タイプバランス・素早さ・データ(ユーザー) |
| iOS のシミュレータ・実機での見た目確認と `make ios-test`。Xcode の署名チーム・実機インストール | state/ios.md・state/type-balance.md Next ④ | 2026-09-26 | iOS(ユーザー) |
| main のブランチ保護の有効化(PR 必須・CI 必須・force push と削除の禁止。AI の自動モードが CI 設定の変更として止めたため人間が実行。他の #243 の設定〈Dependabot・マージ後のブランチ削除・squash/rebase の無効化〉は 2026-10-03 に適用済み) | issue #243・decisions/2026-10-03-data-github-settings-243.md | 2026-10-03 | データ(ユーザー) |
| リポジトリを非公開に戻すか(戻すなら先に Argo CD へ GitHub の認証〈PAT〉を登録しないと Argo CD が取得できなくなる。非公開の無料プランではブランチ保護が使えない) | issue #328・#243 | 2026-10-03 | データ(ユーザー) |
| ソフトウェアアーキテクチャの採用の残りの判断(ヘキサゴナル採用・A は決定済み。S0 の担当レーン・検査の道具・移行の順番ほか 9 問) | docs/plan/architecture-adoption.md §8 | 2026-10-11 | 全レーン(ユーザー) |

## 既定案で進行中(ユーザー未確認)

| 内容 | 場所 | 起票 | 宛先レーン |
|---|---|---|---|
| Wishlist フェーズ4 の既定案(辞書・価格推移・公式の販売状況。通知は作らない〈ユーザー指示〉) | `decisions/2026-10-04-wishlist-phase4-plan.md` | 2026-10-04 | Wishlist |
| Argo CD 管理の gitops overlay が read model を持たず、同期された balance / speed の業務 API が 503 になる(既定案: initContainer で read model を書く) | issue #237 | 2026-09-24 | タイプバランス・素早さ・データ |
| 判定が素早さに効く特性・持ち物・状態異常を無視して `outspeeds` が誤る | issue #235 | 2026-09-24 | 判定 |
| gateway が中断しても calc-svc の逆算が完走する(engine に打ち切り口を入れるか) | issue #498 | 2026-10-02 | API・データ |
| 計算画面で急所・やけど・天候・フィールド・ランク・壁・特性を指定できない(Web・iOS) | issue #274 | 2026-09-25 | Web・iOS |
| 計算・逆算の画面で特性を選べず、防御側の特性が効かない | issue #272 | 2026-10-01 | Web |
| 素早さ「相手を選んで上を取る」の確認点 Q1〜Q7(同速は上を取るに含めない・相手の想定 4 つ・準速○○族以上の向き・届かないときは差だけ・下降補正はトリックルームのみ・表は画面から外す・連続技へ今の振りも渡す。いずれも推奨案で実装済み) | docs/adr/0610-speed-matchup-min-line.md §6 | 2026-10-12 | 素早さ(ユーザー) |

## 未回答の提案

| 内容 | 場所 | 起票 | 宛先レーン |
|---|---|---|---|
| **issue #271・#270 を閉じてよいか**(2026-10-21 データレーン提案)。#271: 段階1〜3(ADR-0142・0143・0144)で攻撃技 335 中 314 を @smogon/calc と同じ値で計算。残り 21 技は意図的に「未対応」の印のまま(対戦の履歴が要る 15〈倍返し・このターンに受けたダメージ等〉・フォルム依存 3・じゅうりょく 1・確率で威力が変わる 1・わるあがき 1。理由は ADR-0144「残す 21 技の理由」)。#270: 補正を持つ持ち物・特性のうち未対応は持ち物 5・特性 25(tools/golden/unsupported-effects.json)で、いずれも「補正未対応」の印が出て黙って等倍にはならない(issue の受け入れ条件「未対応は利用者に分かる」を満たす)。既定案: 両 issue を閉じ、残り(#271 の 21 技の入力設計・#270 の特性 25)は改善要望として docs/plan/improvements/data.md に残す | issue #271・#270・ADR-0144 | 2026-10-21 | データ(ユーザー) |
| pokedex-import の CronJob が Mac の夜間スリープで飛ばされる(startingDeadlineSeconds を長く) | `decisions/2026-10-09-wishlist-cronjob-sleep.md` | 2026-10-09 | データ |

## 直近で回答済み(行は消してよい。2026-10-03 ユーザー「既定案で OK」)

issue #283・#285・#229・#302・#297・#313 と、D31(#225・#253・#256)。決定は `docs/ai-shared/decisions/2026-10-03-data-*.md`。
