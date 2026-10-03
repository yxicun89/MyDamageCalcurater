package main

import (
	"context"
	"net/http"
	"time"
)

// shutdownTimeout は停止時に処理中のリクエストを待つ上限。writeTimeout 以上にして、
// 処理中のリクエストをそれより前に切らない(issue #523、ADR-0804)。Deployment の terminationGracePeriodSeconds は
// preStop + この値より長い(services/gateway/deploytest/prestop_test.go が固定)。
const shutdownTimeout = 10 * time.Second

// gracefulShutdown は新規接続を閉じ、処理中のリクエストを timeout まで待つ。
// 終わらなければ残りの接続を強制的に閉じ、Shutdown の error を返す。呼び出し側はそれをログに残すだけで、異常終了にしない。
func gracefulShutdown(server *http.Server, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := server.Shutdown(ctx)
	if err != nil {
		_ = server.Close()
	}
	return err
}
