package engine

import (
	"math"
	"testing"
)

func rolls16(v int) [16]int {
	var r [16]int
	for i := range r {
		r[i] = v
	}
	return r
}

func TestComputeKOGuaranteedOneHit(t *testing.T) {
	// 全ロール120、HP100 → 確定1発
	ko := ComputeKO(rolls16(120), 100)
	if ko.Hits != 1 || !ko.Guaranteed {
		t.Errorf("ko=%+v want Hits1 Guaranteed", ko)
	}
}

func TestComputeKOGuaranteedTwoHit(t *testing.T) {
	// max90/min76、HP100 → 2発必要、min76*2=152>=100 → 確定2発
	var r [16]int
	for i := 0; i < 16; i++ {
		r[i] = 76 + i // 76..91
	}
	r[15] = 90
	ko := ComputeKO(r, 100)
	if ko.Hits != 2 || !ko.Guaranteed {
		t.Errorf("ko=%+v want Hits2 Guaranteed", ko)
	}
}

func TestComputeKORandomOneHit(t *testing.T) {
	// ロール: 半分80・半分100、HP90 → max100で1発、min80*1=80<90 → 乱数1発
	// 100>=90 のロールは 8/16 → 50%
	var r [16]int
	for i := 0; i < 8; i++ {
		r[i] = 80
	}
	for i := 8; i < 16; i++ {
		r[i] = 100
	}
	ko := ComputeKO(r, 90)
	if ko.Hits != 1 || ko.Guaranteed {
		t.Fatalf("ko=%+v want Hits1 not guaranteed", ko)
	}
	if math.Abs(ko.ChancePercent-50.0) > 1e-9 {
		t.Errorf("chance=%v want 50", ko.ChancePercent)
	}
}

func TestComputeKORandomTwoHit(t *testing.T) {
	// max90/min76、HP170 → 2発、min*2=152<170 → 乱数2発、0<chance<100
	var r [16]int
	for i := 0; i < 16; i++ {
		r[i] = 76 + i
	}
	r[15] = 90
	ko := ComputeKO(r, 170)
	if ko.Hits != 2 || ko.Guaranteed {
		t.Fatalf("ko=%+v want Hits2 not guaranteed", ko)
	}
	if ko.ChancePercent <= 0 || ko.ChancePercent >= 100 {
		t.Errorf("chance=%v want in (0,100)", ko.ChancePercent)
	}
}

func TestComputeKORandomTwoHitExact(t *testing.T) {
	// 2発の合計が HP 以上になる確率を厳密に確認する。
	// ロール: 40 か 60(各 8/16=1/2)、HP=100。
	// 2発合計: (40,40)=80, (40,60)=100, (60,40)=100, (60,60)=120。
	// >=100 になるのは 40+60,60+40,60+60 → 3/4 = 75%
	var r [16]int
	for i := 0; i < 8; i++ {
		r[i] = 40
	}
	for i := 8; i < 16; i++ {
		r[i] = 60
	}
	ko := ComputeKO(r, 100)
	if ko.Hits != 2 || ko.Guaranteed {
		t.Fatalf("ko=%+v want Hits2 random", ko)
	}
	if math.Abs(ko.ChancePercent-75.0) > 1e-9 {
		t.Errorf("chance=%v want 75", ko.ChancePercent)
	}
}

func TestComputeKOBoundaries(t *testing.T) {
	// HP1: どんなダメージでも確定1発
	if ko := ComputeKO(rolls16(50), 1); ko.Hits != 1 || !ko.Guaranteed {
		t.Errorf("HP1 ko=%+v want Hits1 Guaranteed", ko)
	}
	// ちょうど割り切れ + 確定判定が等号ちょうど: max=min=50、HP100 → n=2、50*2==100
	if ko := ComputeKO(rolls16(50), 100); ko.Hits != 2 || !ko.Guaranteed {
		t.Errorf("divisible ko=%+v want Hits2 Guaranteed", ko)
	}
}

func TestComputeKORandomThreeHitExact(t *testing.T) {
	// ロール40/60各1/2、HP160 → n=ceil(160/60)=3、min40*3=120<160 → 乱数3発
	// 3発合計 >=160: (60,60,60)=180が1/8、(60,60,40)=160が 3*(1/8)=3/8 → 合計 4/8=50%
	var r [16]int
	for i := 0; i < 8; i++ {
		r[i] = 40
	}
	for i := 8; i < 16; i++ {
		r[i] = 60
	}
	ko := ComputeKO(r, 160)
	if ko.Hits != 3 || ko.Guaranteed {
		t.Fatalf("ko=%+v want Hits3 random", ko)
	}
	if math.Abs(ko.ChancePercent-50.0) > 1e-9 {
		t.Errorf("chance=%v want 50", ko.ChancePercent)
	}
}

func TestComputeKOCannotKO(t *testing.T) {
	// 無効(全ロール0)→ 倒せない
	ko := ComputeKO(rolls16(0), 100)
	if ko.Hits != 0 {
		t.Errorf("ko=%+v want Hits0", ko)
	}
}

func TestCalcDamagePopulatesKO(t *testing.T) {
	// 統制ケース(base=90、防御HP=175=100+75)。max90 → ceil(175/90)=2発。
	in := ctrlInput([]Type{TypeWater}, []Type{TypePsychic}, CategoryPhysical, TypeNormal)
	r, _ := CalcDamage(in)
	if r.DefenderHP != 175 {
		t.Fatalf("defenderHP=%d want 175", r.DefenderHP)
	}
	if r.KO.Hits != 2 {
		t.Errorf("KO.Hits=%d want 2", r.KO.Hits)
	}
}
