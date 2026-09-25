module example.com/pokecalc/services

go 1.27.1

require (
	example.com/pokecalc/engine v0.0.0
	github.com/getkin/kin-openapi v0.149.0
	github.com/go-sql-driver/mysql v1.10.1
	github.com/golang-migrate/migrate/v4 v4.20.1
	github.com/labstack/echo/v5 v5.3.1
	github.com/nats-io/nats.go v1.54.0
	github.com/oapi-codegen/runtime v1.7.0
	github.com/prometheus/client_golang v1.24.1
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3
	gopkg.in/yaml.v3 v3.0.1
)

require (
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dprotaso/go-yit v0.0.0-20220510233725-9ba8df137936 // indirect
	github.com/go-openapi/jsonpointer v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nats-io/nkeys v0.4.16 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/oapi-codegen/oapi-codegen/v2 v2.8.0 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/speakeasy-api/jsonpath v0.6.3 // indirect
	github.com/speakeasy-api/openapi v1.25.2 // indirect
	github.com/vmware-labs/yaml-jsonpath v0.3.2 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/tools v0.50.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen

replace example.com/pokecalc/engine => ../engine
