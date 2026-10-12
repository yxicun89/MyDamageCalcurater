// Package app は gateway のユースケース層の置き場(ADR-0811)。
//
// gateway は受けた要求を上流のサービスへ転送するだけで、複数の手順を束ねる処理が無いため、今は何も置かない。
// 転送・ルーティング・ヘッダ検証・CORS・画像配信は adapters/httpapi にある。
package app
