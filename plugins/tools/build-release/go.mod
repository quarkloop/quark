module github.com/quarkloop/plugins/tools/build-release

go 1.25.0

require (
	github.com/quarkloop/pkg/plugin v0.0.0
	github.com/quarkloop/pkg/toolkit v0.0.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/cobra v1.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/quarkloop/pkg/plugin v0.0.0 => ../../../pkg/plugin
	github.com/quarkloop/pkg/toolkit v0.0.0 => ../../../pkg/toolkit
)
