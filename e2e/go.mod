module github.com/quarkloop/e2e

go 1.26

require github.com/quarkloop/supervisor v0.0.0

replace (
	github.com/quarkloop/cli v0.0.0 => ../cli
	github.com/quarkloop/runtime v0.0.0 => ../runtime
	github.com/quarkloop/supervisor v0.0.0 => ../supervisor
)
