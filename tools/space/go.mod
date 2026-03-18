module github.com/quarkloop/tools/space

go 1.22

require (
	github.com/quarkloop/agent v0.0.0
	github.com/quarkloop/core v0.0.0
	github.com/spf13/cobra v1.8.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace (
	github.com/quarkloop/agent v0.0.0 => ../../agent
	github.com/quarkloop/core v0.0.0 => ../../core
)
