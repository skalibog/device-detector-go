module github.com/skalibog/device-detector-go/examples/sievecache

go 1.24.11

toolchain go1.24.13

require (
	github.com/guerinoni/sieve v1.1.2
	github.com/skalibog/device-detector-go v1.0.1
)

require (
	github.com/dlclark/regexp2 v1.12.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/skalibog/device-detector-go => ../..
