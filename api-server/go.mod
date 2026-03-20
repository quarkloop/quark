module github.com/quarkloop/api-server

go 1.22

require (
	github.com/quarkloop/agent v0.0.0
	github.com/quarkloop/agent-api v0.0.0
	github.com/quarkloop/agent-client v0.0.0
	github.com/quarkloop/core v0.0.0
	github.com/quarkloop/tools/space v0.0.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/quarkloop/agent v0.0.0 => ../agent
	github.com/quarkloop/agent-api v0.0.0 => ../agent-api
	github.com/quarkloop/agent-client v0.0.0 => ../agent-client
	github.com/quarkloop/core v0.0.0 => ../core
	github.com/quarkloop/tools/space v0.0.0 => ../tools/space
)
