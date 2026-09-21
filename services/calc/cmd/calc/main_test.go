package main

// calc-svc の起動(設定・マスタの読み込み)の受け入れテスト(ADR-0200 AC-10)。
// 例のマスタ(架空データ)と共有の相性表(testdata/golden/typechart.json)で起動できること、
// 必須の環境変数の欠落・壊れたファイルでは起動しないこと。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	exampleMasterPath   = "../../testdata/master.example.json"
	sharedTypeChartPath = "../../../../testdata/golden/typechart.json"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func validEnv() map[string]string {
	return map[string]string{
		envMasterPath:    exampleMasterPath,
		envTypeChartPath: sharedTypeChartPath,
	}
}

// 環境変数の名前は運用(k8s の manifest・README)が依存するので固定する。
func TestEnvNames(t *testing.T) {
	if envAddr != "CALC_ADDR" || envMasterPath != "CALC_MASTER_PATH" || envTypeChartPath != "CALC_TYPECHART_PATH" {
		t.Fatalf("環境変数名 = %q %q %q", envAddr, envMasterPath, envTypeChartPath)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    config
		wantErr bool
	}{
		{"既定の待ち受けアドレス", validEnv(), config{Addr: ":8080", MasterPath: exampleMasterPath, TypeChartPath: sharedTypeChartPath}, false},
		{"CALC_ADDR が空なら既定", func() map[string]string { e := validEnv(); e[envAddr] = ""; return e }(),
			config{Addr: ":8080", MasterPath: exampleMasterPath, TypeChartPath: sharedTypeChartPath}, false},
		{"CALC_ADDR の指定", func() map[string]string { e := validEnv(); e[envAddr] = "127.0.0.1:9090"; return e }(),
			config{Addr: "127.0.0.1:9090", MasterPath: exampleMasterPath, TypeChartPath: sharedTypeChartPath}, false},
		{"CALC_MASTER_PATH が無い", func() map[string]string { e := validEnv(); delete(e, envMasterPath); return e }(), config{}, true},
		{"CALC_MASTER_PATH が空", func() map[string]string { e := validEnv(); e[envMasterPath] = ""; return e }(), config{}, true},
		{"CALC_TYPECHART_PATH が無い", func() map[string]string { e := validEnv(); delete(e, envTypeChartPath); return e }(), config{}, true},
		{"CALC_TYPECHART_PATH が空", func() map[string]string { e := validEnv(); e[envTypeChartPath] = ""; return e }(), config{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(lookupFrom(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("loadConfig = %+v, nil; want エラー", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("loadConfig = %+v, %v; want %+v, nil", got, err, tt.want)
			}
		})
	}
}

// 例のマスタで起動したハンドラが、/healthz と計算に答える(端から端まで。マスタの実体は架空データ)。
func TestNewHandlerServesExampleMaster(t *testing.T) {
	cfg, err := loadConfig(lookupFrom(validEnv()))
	if err != nil {
		t.Fatalf("loadConfig = %v", err)
	}
	h, err := newHandler(cfg)
	if err != nil {
		t.Fatalf("newHandler = %v", err)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz status = %d, want 200", rec.Code)
	}

	// master.example.json の ID(テストモン → テストガード、テストビーム)。
	body := `{"format":"single",` +
		`"attacker":{"speciesKey":"9001-000","natureId":"test-atk-up","sp":{"hp":0,"atk":32,"def":0,"spa":0,"spd":0,"spe":32}},` +
		`"defender":{"speciesKey":"9002-000","natureId":"test-neutral-a","sp":{"hp":32,"atk":0,"def":0,"spa":0,"spd":0,"spe":0}},` +
		`"moveId":"test-beam"}`
	req := httptest.NewRequest(http.MethodPost, "/api/calc", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Id", "00000000-0000-4000-8000-000000000001")
	req.Header.Set("X-Session-Id", "00000000-0000-4000-8000-000000000002")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/calc status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var res struct {
		Rolls []int `json:"rolls"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || len(res.Rolls) != 16 || res.Rolls[15] <= 0 {
		t.Fatalf("計算結果が不正: %v; body=%s", err, rec.Body.String())
	}
}

// 起動前の失敗: ファイルが無い・壊れている・スキーマ違反なら newHandler も run もエラー(非ゼロ終了の元)。
func TestStartupFailsOnBadFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	broken := write("broken.json", `{"schemaVersion":1,`)
	wrongSchema := write("wrong.json", `{"schemaVersion":1,"unknown":true}`)
	missing := filepath.Join(dir, "missing.json")

	tests := []struct {
		name          string
		env           map[string]string
		wantConfigErr bool // true: 設定の段で落ちる / false: 設定は通り、ファイルの読み込みで落ちる
	}{
		{"マスタが無い", map[string]string{envMasterPath: missing, envTypeChartPath: sharedTypeChartPath}, false},
		{"マスタが壊れている", map[string]string{envMasterPath: broken, envTypeChartPath: sharedTypeChartPath}, false},
		{"マスタがスキーマ違反", map[string]string{envMasterPath: wrongSchema, envTypeChartPath: sharedTypeChartPath}, false},
		{"相性表が無い", map[string]string{envMasterPath: exampleMasterPath, envTypeChartPath: missing}, false},
		{"相性表が壊れている", map[string]string{envMasterPath: exampleMasterPath, envTypeChartPath: broken}, false},
		{"相性表のパスにマスタを渡した", map[string]string{envMasterPath: exampleMasterPath, envTypeChartPath: exampleMasterPath}, false},
		{"必須の環境変数が無い", map[string]string{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := loadConfig(lookupFrom(tt.env))
			if tt.wantConfigErr {
				if err == nil {
					t.Fatalf("loadConfig = %+v, nil; want エラー", cfg)
				}
			} else {
				if err != nil {
					t.Fatalf("loadConfig = %v; want nil(パスはそろっている)", err)
				}
				if h, err := newHandler(cfg); err == nil || h != nil {
					t.Errorf("newHandler = %v, %v; want nil, エラー", h, err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			env := tt.env
			env[envAddr] = "127.0.0.1:0"
			if err := run(ctx, lookupFrom(env)); err == nil {
				t.Error("run = nil, want 起動エラー")
			}
		})
	}
}

// 正常な設定なら run は待ち受け、ctx の終了で止まって nil を返す。
func TestRunStopsOnContextCancel(t *testing.T) {
	env := validEnv()
	env[envAddr] = "127.0.0.1:0"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, lookupFrom(env)) }()

	// 起動直後の失敗(設定・マスタ)はすぐ返るので、少し待ってまだ動いていることを確かめる。
	select {
	case err := <-done:
		t.Fatalf("run が ctx の終了前に返った: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run = %v, want nil(正常な停止)", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run が ctx の終了後 5 秒以内に止まらない")
	}
}
