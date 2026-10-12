// Package app は record-svc のユースケース層(ADR-0811・ADR-0254)。
//
// 端末の利用の記録(TouchDevice。ADR-0209 §4)を先に行ってから、各操作(よく使う相手・端末データの削除・
// お気に入りの一覧/作成/削除・計算履歴)の手順を進める。失効ジョブの手順は app/expire。
// record DB のポート(Store・Pinger)はここで宣言し、実装は adapters/store が渡す。
// 生成型(services/internal/api)・HTTP・DB ドライバ・NATS を知らない。失敗は store の失敗(ErrUnavailable ほか)と
// domain.Error をそのまま返し、HTTP への写しとログは adapters/httpapi が行う。
package app
