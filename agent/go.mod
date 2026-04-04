module github.com/quarkloop/agent

go 1.24.0

require (
	github.com/google/uuid v1.6.0
	github.com/quarkloop/agent-api v0.0.0
	github.com/quarkloop/cli v0.0.0
	github.com/quarkloop/core v0.0.0
	github.com/spf13/cobra v1.8.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace (
	github.com/quarkloop/agent-api v0.0.0 => ../agent-api
	github.com/quarkloop/cli v0.0.0 => ../cli
	github.com/quarkloop/core v0.0.0 => ../core
)
