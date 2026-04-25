module github.com/quarkloop/plugins/tools/bash

go 1.25.0

require (
	github.com/quarkloop/pkg/plugin v0.0.0
	github.com/quarkloop/pkg/toolkit v0.0.0
)

replace (
	github.com/quarkloop/pkg/plugin v0.0.0 => ../../../pkg/plugin
	github.com/quarkloop/pkg/toolkit v0.0.0 => ../../../pkg/toolkit
)
