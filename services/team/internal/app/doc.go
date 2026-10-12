// Package app は team-svc のユースケース層(ADR-0811・ADR-0255)。
//
// 端末の利用の記録(TouchDevice。ADR-0209 §4)を先に行ってから、各操作(構築の一覧/取得/作成/置換/削除・
// 端末データの削除)の手順を進める。作成・置換は domain の検証を通してから保存する。失効ジョブの手順は app/expire。
// team DB のポート(Store・Pinger)はここで宣言し、実装は adapters/store が渡す。
// 生成型(services/internal/api)・HTTP・DB ドライバ・NATS を知らない。失敗は store の失敗(ErrUnavailable ほか)と
// domain.Error をそのまま返し、HTTP への写しとログは adapters/httpapi が行う。
package app
