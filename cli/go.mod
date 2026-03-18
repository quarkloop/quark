module github.com/quarkloop/cli

go 1.22

require (
	github.com/quarkloop/agent v0.0.0
	github.com/quarkloop/api-server v0.0.0
	github.com/quarkloop/space v0.0.0
	github.com/spf13/cobra v1.8.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/quarkloop/kb v0.0.0 // indirect
	github.com/quarkloop/store v0.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/quarkloop/agent v0.0.0 => ../agent
	github.com/quarkloop/api-server v0.0.0 => ../api-server
	github.com/quarkloop/kb v0.0.0 => ../kb
	github.com/quarkloop/space v0.0.0 => ../space
	github.com/quarkloop/store v0.0.0 => ../store
)
