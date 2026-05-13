module github.com/quarkloop/cli

go 1.26.2

require (
	github.com/quarkloop/pkg/space v0.0.0
	github.com/quarkloop/runtime v0.0.0
	github.com/quarkloop/supervisor v0.0.0
	github.com/spf13/cobra v1.8.0
)

require gopkg.in/yaml.v3 v3.0.1 // indirect

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/quarkloop/pkg/event v0.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace (
	github.com/quarkloop/pkg/event => ../pkg/event
	github.com/quarkloop/pkg/plugin => ../pkg/plugin
	github.com/quarkloop/pkg/serviceapi => ../pkg/serviceapi
	github.com/quarkloop/pkg/space v0.0.0 => ../pkg/space
	github.com/quarkloop/runtime v0.0.0 => ../runtime
	github.com/quarkloop/services/space => ../services/space
	github.com/quarkloop/supervisor v0.0.0 => ../supervisor
)
