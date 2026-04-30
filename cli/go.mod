module github.com/quarkloop/cli

go 1.25.0

require (
	github.com/quarkloop/agent v0.0.0
	github.com/quarkloop/pkg/space v0.0.0
	github.com/quarkloop/supervisor v0.0.0
	github.com/spf13/cobra v1.8.0
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace (
	github.com/quarkloop/agent v0.0.0 => ../agent
	github.com/quarkloop/pkg/space v0.0.0 => ../pkg/space
	github.com/quarkloop/supervisor v0.0.0 => ../supervisor
)
