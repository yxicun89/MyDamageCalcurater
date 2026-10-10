package importer_test

// 持ち物のなげつけるの威力(Showdown の fling.basePower → items.fling_power。ADR-0144 §4)の取り込み。
//
// 受け入れ条件:
//   - tools/importer/fetch-showdown.mjs は持ち物ごとに flingBasePower(取得元の fling.basePower。投げられない持ち物は null)を出す。
//     キーが無い古い取得物は DecodeShowdownSnapshot が ErrInvalidInput(`make import-fetch` で取り直す)。
//   - Convert は持ち物の行(NamedRow)の FlingPower に写す(null は 0 = NULL)。1..255 の整数以外は ErrInvalidData。
//   - fixture(testdata/fictional)の取得物は flingBasePower を持つ(実装の手順で足す: testmonite 80・testorb 30・testberry 10・
//     testrelic null)。このテストは fixture に依存しないよう、取得物を書き換えてから読む。

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

// editShowdownItems は Showdown の取得物の持ち物の flingBasePower を書き換えてデコードし直す。
// values の値が "" のときはキーを消す(古い取得物)。values に無い持ち物は null にする。
func editShowdownItems(t *testing.T, in importer.Input, values map[string]string) ([]byte, error) {
	t.Helper()
	b, err := json.Marshal(in.Showdown)
	if err != nil {
		t.Fatalf("取得物を JSON にできない: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	for _, v := range doc["items"].([]any) {
		it := v.(map[string]any)
		raw, ok := values[it["id"].(string)]
		switch {
		case !ok:
			it["flingBasePower"] = nil
		case raw == "":
			delete(it, "flingBasePower")
		default:
			it["flingBasePower"] = json.RawMessage(raw)
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return out, nil
}

func itemFlingPowers(out importer.Output) map[string]int {
	m := map[string]int{}
	for _, it := range out.Items {
		m[it.ID] = it.FlingPower
	}
	return m
}

func TestConvertItemFlingPower(t *testing.T) {
	in := loadFixture(t)
	raw, _ := editShowdownItems(t, in, map[string]string{"testmonite": "80", "testorb": "30", "testberry": "10"})
	sd, err := importer.DecodeShowdownSnapshot(raw)
	if err != nil {
		t.Fatalf("DecodeShowdownSnapshot: %v", err)
	}
	in.Showdown = sd
	out, _ := convertOK(t, in)
	got := itemFlingPowers(out)
	for id, want := range map[string]int{"testmonite": 80, "testorb": 30, "testberry": 10} {
		if v, ok := got[id]; !ok || v != want {
			t.Errorf("%s の FlingPower = %d(取り込み %v), want %d", id, v, ok, want)
		}
	}
}

func TestConvertItemFlingPowerNullIsZero(t *testing.T) {
	in := loadFixture(t)
	raw, _ := editShowdownItems(t, in, map[string]string{})
	sd, err := importer.DecodeShowdownSnapshot(raw)
	if err != nil {
		t.Fatalf("DecodeShowdownSnapshot: %v", err)
	}
	in.Showdown = sd
	out, _ := convertOK(t, in)
	for id, v := range itemFlingPowers(out) {
		if v != 0 {
			t.Errorf("%s: null の flingBasePower は 0(NULL)のはず: %d", id, v)
		}
	}
}

func TestConvertItemFlingPowerRejects(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"0", "0"}, {"負", "-10"}, {"小数", "30.5"}, {"上限超え", "256"}, {"文字列", `"30"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := loadFixture(t)
			raw, _ := editShowdownItems(t, in, map[string]string{"testorb": tc.raw})
			sd, err := importer.DecodeShowdownSnapshot(raw)
			if err != nil {
				// デコードで拒否してもよい(取得物の形の誤り)。
				if !errors.Is(err, importer.ErrInvalidInput) && !errors.Is(err, importer.ErrInvalidData) {
					t.Fatalf("DecodeShowdownSnapshot: %v", err)
				}
				return
			}
			in.Showdown = sd
			if _, _, err := importer.Convert(in); !errors.Is(err, importer.ErrInvalidData) {
				t.Fatalf("err = %v, want ErrInvalidData", err)
			}
		})
	}
}

func TestDecodeShowdownSnapshotRequiresFlingBasePower(t *testing.T) {
	in := loadFixture(t)
	raw, _ := editShowdownItems(t, in, map[string]string{"testorb": ""})
	_, err := importer.DecodeShowdownSnapshot(raw)
	if !errors.Is(err, importer.ErrInvalidInput) || !strings.Contains(err.Error(), "flingBasePower") {
		t.Fatalf("err = %v, want ErrInvalidInput(flingBasePower と取り直しを案内)", err)
	}
}
