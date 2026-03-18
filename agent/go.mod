module github.com/quarkloop/agent

go 1.22

require (
	github.com/google/uuid v1.6.0
	github.com/quarkloop/core v0.0.0
	github.com/spf13/cobra v1.8.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace github.com/quarkloop/core v0.0.0 => ../core
