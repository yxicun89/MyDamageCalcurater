package main

// AC-K5(ADR-0220 §1): gateway は base(と local)で record-svc・team-svc の Service を上流として指す
// (Service 名はクラウドでも同じなので base に置く。ADR-0206 §1 と同じ理由)。未設定のままだと
// /api/record/*・/api/team/* は常に 503 upstream_unavailable で、k3d から届かない(P5-3b・P5-4b の主題)。

import (
	"testing"

	"example.com/pokecalc/services/gateway/deploytest"
)

const (
	recordServiceName = "record"
	teamServiceName   = "team"
)

func TestManifestGatewayRecordTeamURLs(t *testing.T) {
	for _, tc := range []struct {
		name string
		d    deploytest.Deployment
	}{
		{"base", deploytest.BaseDeployment(t, gatewayService)},
		{"local", deploytest.LocalDeployment(t, gatewayService)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := tc.d.Container(t, gatewayService).EnvMap(t)
			cfg, err := loadConfig(lookupFrom(env))
			if err != nil {
				t.Fatalf("loadConfig: %v(env=%v)", err, env)
			}
			assertServiceURL(t, envRecordURL, cfg.Gateway.RecordURL, recordServiceName)
			assertServiceURL(t, envTeamURL, cfg.Gateway.TeamURL, teamServiceName)
		})
	}
}
