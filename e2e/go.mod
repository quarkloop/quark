module github.com/quarkloop/e2e

go 1.24.0

require (
	github.com/quarkloop/agent v0.0.0
	github.com/quarkloop/supervisor v0.0.0
)

replace (
	github.com/quarkloop/agent v0.0.0 => ../agent
	github.com/quarkloop/supervisor v0.0.0 => ../supervisor
	github.com/quarkloop/cli v0.0.0 => ../cli
)
