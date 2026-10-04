## ブロッカー
解決済みの記録は [plan-archive.md](../plan-archive.md)。未解決のものだけをここに置く(issue があるものは issue を正とする)。


**【人間の確認待ち】**
- **失効ジョブ(record-expire・team-expire)を k3d の実データへ初めて向ける承認**(ADR-0209 の「人間の確認」・ADR-0220 未決事項 0。P5-3b・P5-4b): 既定案は「承認までは local・cloud とも CronJob を `suspend: true` のまま(自動で実データを消さない)」。承認後に `deploy/k8s/overlays/local/cronjob-expire-suspend-patch.yaml` と kustomization の `patches` を外す。承認前の確認は手動 Job(`kubectl -n pokecalc create job --from=cronjob/record-expire record-expire-manual-...`。docs/runbooks/api.md §8)。作業は止まらない。
- **P4-5 の Safari 実機確認**(仕様ブロッカーではない。作業は止めない。Chrome は 2026-09-22 に確認済み): `make web-dev` で開き、Safari で計算・逆算が動くこと、`.wasm` の MIME type(`application/wasm`)・`WebAssembly.instantiateStreaming`(失敗時は arrayBuffer にフォールバック)・キャッシュ・初回ロード(約4.6MB / gzip 1.3MB)・メモリを確認する。加えて issue #333: 375px 幅未満でタブ列を左端までスクロールし、先頭の「計算」タブが読める・押せること(`justify-content: safe center` の Safari 対応)。確認できるまで P4-5 は「実装・自動テスト済み、Safari 実機未確認」として扱う。

- **観測%の丸め方**(逆算の入力側。ADR-0010 §R2): 実機の相手 HP の減少は整数%で表示される。丸め方(切り捨てか四捨五入か)だけが未確認。確認できるまでは、どの丸めでも真値を落とさない区間で照合する(P1-12 で実装済み。作業は止まらない)。確認できたら `Observation.Matches` の1か所で区間を狭める。確認方法の例: HP が分かっている自分のポケモンで、実点数のダメージと画面の%を見比べる。

- **公開のタイミング**(R-2-9・LICENSE・issue #328): 公開するときに、クリーンコピーの作成と、第三者データを含まない状態の確認、LICENSE の決定を行う。それまでは今のリポジトリで開発を続ける。
