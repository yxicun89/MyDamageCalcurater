## 2026-09-23: issue #103 の設計を ADR-0209 で確定(API レーン。critic PASS(NG 2回のあと3回目))
Decision: M2 保存データの保持・削除・端末ID境界を ADR-0209 で確定した。端末IDは認証ではなくデータの分割キー(セッションIDは
分割キーにしない) / v1 は個人利用+Tailscale 内に固定し公開前に認証を別ADRで必須決定 / 生の計算イベントは作成から90日・推薦の
集計は生イベントと同時に失効・構築とお気に入りは `max(devices.last_seen_at, 行.updated_at)` から540日(record-svc・team-svc
どちらも calc-svc の計算イベントを購読して自分の DB の `devices.last_seen_at` を更新する。record 側だけでは計算だけ使い続け
構築画面を開かない端末の team データが誤って失効するため) / 端末単位の全削除はサービスごとに1本
(DELETE /api/record/device-data・DELETE /api/team/device-data。冪等・同期・partial の繰り返し・`purged_at` は削除要求のたびに
現在時刻へ更新) / 削除の墓石 `devices.purged_at` で JetStream の遅延イベントの復活を防ぎ、purge journal(#5b。DB のバックアップ
世代とは独立の保存先に同時追記)でバックアップ世代取得後に来た削除要求もリストア時に再適用する。
openapi.yaml は端末ID/セッションIDの description だけ変更し、record/team のパスは P5-3/P5-4 で入れる(単一の
api.ServerInterface のため、今足すと calc-svc/pokedex-svc に常に404の空メソッドが増え、gateway も未ルーティングで常に404になる)。
Reason: ユーザー決定(2026-09-23「一定期間で自動失効。無期限保持はしない」)の具体化。issue #103 の受け入れ条件。
Impact(データレーン): P5-1 の TiDB スキーマは ADR-0209 §3 に従う。devices テーブル(last_seen_at・purged_at)と purge journal
テーブルを record DB と team DB にそれぞれ持ち、purge journal は DB とは別の独立した保存先(P7-4 が決める)にも同時に書く。
全表に device_id を置く。favorites は calc_events を外部キーで参照せず個体スナップショットを自分で持つ(90日と540日の差が
矛盾するため)。保持日数はコードに埋めず環境変数で渡し起動時に検証する。P5-2 はストリームの max_age を7日にし、イベントに
発生時刻 occurred_at を載せ、record-svc と team-svc は別々の durable consumer を持つ(同じ consumer を共有すると配送が分かれ
record-svc が計算イベントを取りこぼす)。P7-4 はバックアップに devices(墓石)と purge journal を必ず含め、復元は Ready の前に
墓石の再適用・purge journal の再適用・失効ジョブの強制実行を行い、JetStream は再生しない。
Impact(Web/iOS レーン): ADR-0209 §8 の文言と削除 UI をお願いしたい(Web は P5-5、iOS は P6-5)。(1)「アカウントはありません。
履歴・お気に入り・構築はこの端末に割り当てた ID でサーバーに保存しています」(2)「ID が変わると(Web: サイトデータ消去 / iOS:
アプリの再インストール)前のデータは開けません。元に戻す方法はありません」(3)「開けなくなったデータは自動的に消えます。計算の
履歴は記録から90日、お気に入りと構築は最後に使った日から18か月です」(4) ボタン「この端末のデータを削除」→ 確認「元に戻せません」
→ record と team の両方が completed になってから「削除しました」。partial は続けて再送、503 は「サーバーに届きませんでした」。
API が実装されるまでは文言と画面だけ先に置いてよい。
