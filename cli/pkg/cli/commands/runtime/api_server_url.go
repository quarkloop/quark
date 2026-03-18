package runtime

import "github.com/quarkloop/cli/pkg/cli/config"

// apiServerURL returns the api-server base URL.
func apiServerURL() string {
	return config.APIServerURL()
}
