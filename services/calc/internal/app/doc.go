// Package app は calc-svc のユースケース層(ADR-0811・ADR-0252)。
//
// 計算(Calc)・一括(Bulk)・逆算(Reverse)・連続技(Combo。イベントは発行しない。ADR-0253)の 1 操作の手順を持つ:
//
//	列挙の検証(invalid_enum)→ ID の解決(unknown_*)→ 入力の検証(invalid_input)→ 締め切りの確認
//	→ engine 呼び出し → イベント発行
//
// 失敗の順番は HTTP 境界にあったときと同じに保つ(同じ入力は同じ code。engine/wasmapi と揃える。ADR-0200)。
// マスタの参照(Master)・イベント発行(EventPublisher)・締め切りの確認(Expired)はここで宣言するポートで、
// 実装は adapters(master・events・httpapi)と cmd が渡す。生成型(services/internal/api)・HTTP・NATS を知らない。
//
// 調整(adjust)の手順はまだ adapters/httpapi にある(S3 の後半の PR で移す。docs/plan/architecture-adoption.md §6)。
package app
