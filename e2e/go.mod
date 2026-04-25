module github.com/quarkloop/e2e

go 1.25.0

require github.com/quarkloop/supervisor v0.0.0

replace (
	github.com/quarkloop/agent v0.0.0 => ../agent
	github.com/quarkloop/cli v0.0.0 => ../cli
	github.com/quarkloop/supervisor v0.0.0 => ../supervisor
)
