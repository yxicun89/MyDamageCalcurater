// Package domain は gateway の純粋な核の置き場(ADR-0811)。
//
// gateway はルーティング・ヘッダ検証・CORS・転送だけを行い、業務の規則を持たない。
// そのためここに置くものは今は無い(中身の無いフォルダも doc.go で役割を書く。architecture-adoption §8 Q7)。
// 持つことになっても HTTP・DB・時刻の取得は持ち込まない。
package domain
