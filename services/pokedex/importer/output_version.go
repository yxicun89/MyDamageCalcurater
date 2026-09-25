package importer

// 変換結果(Output)の内容ハッシュを「取得元の版」と同じ data_versions の1行として記録する
// (issue #379・ADR-0122)。取得元の版が同じでも、importer の変換ロジックや DB スキーマの変更で
// 変換結果が変われば、次の取り込みで投入する。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
)

// OutputSource は変換結果の版を記録する data_versions の source 名。取得元の名前と重ならないこと
// (重なれば validateVersions が ErrInvalidInput にする)。
const OutputSource = "importer-output"

// outputVersionLength は version 列に入れるハッシュの先頭の長さ(dataVersion の表示用。
// 判定には checksum 列の全長を使う)。
const outputVersionLength = 12

// outputHashFormat はハッシュの直列化の形式の版。形式を変えたら上げる(全件が再投入になる)。
const outputHashFormat = "importer-output/v1"

// OutputVersion は out の内容ハッシュを SourceVersion として返す。
// 直列化は決定的にする: 表(Output のフィールド)ごとに、行を JSON にして文字列順に並べてから
// 表の名前・行数と一緒に sha256 に流す。行の並び順は DB の中身を変えないので、ハッシュにも効かせない。
// JSON は map のキーを整列し、[]byte を base64 にするので、同じ内容なら同じバイト列になる。
func OutputVersion(out Output) (SourceVersion, error) {
	h := sha256.New()
	if _, err := io.WriteString(h, outputHashFormat+"\n"); err != nil {
		return SourceVersion{}, err
	}
	v := reflect.ValueOf(out)
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		rows, err := canonicalRows(v.Field(i))
		if err != nil {
			return SourceVersion{}, fmt.Errorf("変換結果 %s を直列化できない: %w", name, err)
		}
		if _, err := fmt.Fprintf(h, "%s %d\n", name, len(rows)); err != nil {
			return SourceVersion{}, err
		}
		for _, r := range rows {
			// JSON は改行を含まない(文字列中の改行はエスケープされる)ので、行の境界はあいまいにならない。
			if _, err := fmt.Fprintf(h, "%s\n", r); err != nil {
				return SourceVersion{}, err
			}
		}
	}
	sum := hex.EncodeToString(h.Sum(nil))
	return SourceVersion{Source: OutputSource, Version: sum[:outputVersionLength], Checksum: sum}, nil
}

// canonicalRows は表の各行を JSON にして並べ替えたものを返す。slice でないフィールドは1行として扱う。
func canonicalRows(field reflect.Value) ([]string, error) {
	if field.Kind() != reflect.Slice {
		raw, err := json.Marshal(field.Interface())
		if err != nil {
			return nil, err
		}
		return []string{string(raw)}, nil
	}
	rows := make([]string, 0, field.Len())
	for i := 0; i < field.Len(); i++ {
		raw, err := json.Marshal(field.Index(i).Interface())
		if err != nil {
			return nil, err
		}
		rows = append(rows, string(raw))
	}
	slices.Sort(rows)
	return rows, nil
}

// WithOutputVersion は取得元の版 versions の末尾に out の版を足した新しい slice を返す
// (versions は変えない)。
func WithOutputVersion(versions []SourceVersion, out Output) ([]SourceVersion, error) {
	ov, err := OutputVersion(out)
	if err != nil {
		return nil, err
	}
	return append(slices.Clone(versions), ov), nil
}
