module example.com/pokecalc/services/judge

go 1.27.1

require (
	example.com/pokecalc/engine v0.0.0
	github.com/labstack/echo/v5 v5.3.1
	github.com/oapi-codegen/runtime v1.7.0
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
)

replace example.com/pokecalc/engine => ../../engine
