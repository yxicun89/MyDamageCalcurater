package calcevents

import (
	"testing"

	"example.com/pokecalc/services/internal/api"
)

// TestContractFieldsExist は api/openapi.yaml から生成される型の、この package が前提にしている
// フィールドが存在することを固定する(ADR-0212 §7・AC-N8)。CalcDetail は api.Individual・
// api.FieldState・api.CalcOptions を丸ごと埋め込むが、値を写す側(calc-svc のハンドラ)は
// api.CalcRequest.MoveId(トップレベル。Individual.MoveId ではない)・
// api.CalcResult.MinPercent/MaxPercent という*個別のフィールド*に依存する。
// これらが openapi.yaml の変更でリネーム・削除されると、この関数のフィールドアクセスが
// コンパイルできなくなり、ビルド時に検知できる(services/internal/api/client_id_semantics_test.go
// と同じ「契約が壊れたらビルドで気づく」発想。実行時のアサーションは無くてよい)。
func TestContractFieldsExist(t *testing.T) {
	var req api.CalcRequest
	var result api.CalcResult

	// このテスト自体はフィールドが存在すればコンパイルが通り、それだけで目的を達成する
	// (openapi.yaml の変更でこれらのフィールドが消える・リネームされると go build/go test の
	// コンパイル段階で失敗する)。
	var detail CalcDetail
	detail.MoveID = req.MoveId // トップレベルの moveId。req.Attacker.MoveId ではない。
	detail.MinPercent = result.MinPercent
	detail.MaxPercent = result.MaxPercent
	detail.Attacker = req.Attacker
	detail.Defender = req.Defender
	detail.Field = req.Field
	detail.Options = req.Options
}
