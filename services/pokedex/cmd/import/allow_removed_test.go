package main

// -allow-removed(ID の消滅を人が承認するフラグ)と、消滅で止まったときの終了コード・案内
// (issue #277・ADR-0131)。DB は偽の Store。

import (
	"reflect"
	"strings"
	"testing"

	"example.com/pokecalc/services/pokedex/importer"
)

func TestRunPassesAllowRemovedToApply(t *testing.T) {
	data := copyFixtureData(t, 2)
	h := newHarness()
	if code := h.run(t, "-data", data, "-allow-removed", "species:9002-002,move:teststrike"); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	want := importer.ApplyOptions{AllowRemoved: []importer.RemovedID{
		{Kind: importer.IDKindSpecies, ID: "9002-002"},
		{Kind: importer.IDKindMove, ID: "teststrike"},
	}}
	if len(h.store.applyOpts) != 1 || !reflect.DeepEqual(h.store.applyOpts[0], want) {
		t.Fatalf("Apply に渡った opts = %+v, want [%+v]", h.store.applyOpts, want)
	}
}

// 既定(フラグなし)は何も許さない(CronJob は消滅を自動で通さない)。
func TestRunWithoutAllowRemovedAllowsNothing(t *testing.T) {
	data := copyFixtureData(t, 2)
	h := newHarness()
	if code := h.run(t, "-data", data); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(h.store.applyOpts) != 1 || len(h.store.applyOpts[0].AllowRemoved) != 0 {
		t.Fatalf("フラグなしなのに許可が渡った: %+v", h.store.applyOpts)
	}
}

// 形式の誤りは使い方の誤り(終了コード 2)。DB を開かない。
func TestRunRejectsMalformedAllowRemoved(t *testing.T) {
	for _, v := range []string{"9002-002", "pokemon:9002-002", "species:"} {
		t.Run(v, func(t *testing.T) {
			data := copyFixtureData(t, 2)
			h := newHarness()
			if code := h.run(t, "-data", data, "-allow-removed", v); code != 2 {
				t.Fatalf("exit = %d, want 2", code)
			}
			if h.openCalls != 0 {
				t.Errorf("フラグの誤りなのに DB を開いた(%d 回)", h.openCalls)
			}
		})
	}
}

// 消滅で止まったら終了コード 3(人の対応が要る)。stderr に消えた ID を <種類>:<ID> で並べ、
// -allow-removed での承認の仕方を案内する。
func TestRunKeyRemovedExitsNeedsHuman(t *testing.T) {
	data := copyFixtureData(t, 2)
	h := newHarness()
	h.store.applyErr = &importer.RemovedIDsError{IDs: []importer.RemovedID{
		{Kind: importer.IDKindMove, ID: "teststrike"},
		{Kind: importer.IDKindSpecies, ID: "9002-002"},
	}}
	if code := h.run(t, "-data", data); code != 3 {
		t.Fatalf("exit = %d, want 3", code)
	}
	stderr := h.stderr.String()
	for _, want := range []string{"move:teststrike", "species:9002-002", "-allow-removed"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr に %q が無い:\n%s", want, stderr)
		}
	}
}

func TestClassifyErrKeyRemoved(t *testing.T) {
	if got := classifyErr(importer.ErrKeyRemoved); got != 3 {
		t.Errorf("classifyErr(ErrKeyRemoved) = %d, want 3", got)
	}
	if got := classifyErr(&importer.RemovedIDsError{IDs: []importer.RemovedID{{Kind: importer.IDKindItem, ID: "testorb"}}}); got != 3 {
		t.Errorf("classifyErr(*RemovedIDsError) = %d, want 3", got)
	}
}
