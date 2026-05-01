module github.com/quarkloop/plugins/providers/openrouter

go 1.25.0

require github.com/quarkloop/pkg/plugin v0.0.0

require (
	github.com/kr/text v0.2.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/quarkloop/pkg/plugin v0.0.0 => ../../../pkg/plugin
	github.com/quarkloop/pkg/toolkit v0.0.0 => ../../../pkg/toolkit
)
