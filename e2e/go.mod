module github.com/quarkloop/e2e

go 1.24.0

require (
	github.com/quarkloop/agent-api v0.0.0
	github.com/quarkloop/agent-client v0.0.0
)

replace (
	github.com/quarkloop/agent v0.0.0 => ../agent
	github.com/quarkloop/agent-api v0.0.0 => ../agent-api
	github.com/quarkloop/agent-client v0.0.0 => ../agent-client
	github.com/quarkloop/cli v0.0.0 => ../cli
	github.com/quarkloop/core v0.0.0 => ../core
)
