module github.com/skalibog/device-detector-go

go 1.22

require (
	github.com/dlclark/regexp2 v1.12.0
	gopkg.in/yaml.v3 v3.0.1
)

// Tagged before the repository rename; its go.mod declares the old module
// path (github.com/skalibog/devicedetector) and cannot be fetched.
retract v0.1.0
