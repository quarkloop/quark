module github.com/quarkloop/pkg/toolkit

go 1.25.0

require (
	github.com/quarkloop/pkg/plugin v0.0.0
	github.com/spf13/cobra v1.8.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/quarkloop/pkg/plugin v0.0.0 => ../plugin
