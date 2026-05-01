// Package agentdial provides helpers for connecting the CLI to the agent
// process of the current space. All lookups go through the supervisor —
// the CLI never reads the filesystem to discover a running agent.
package agentdial

import (
	"context"
	"fmt"

	agentclient "github.com/quarkloop/runtime/pkg/client"
	spacemodel "github.com/quarkloop/pkg/space"
	"github.com/quarkloop/supervisor/pkg/api"
	supclient "github.com/quarkloop/supervisor/pkg/client"
)

// Current resolves the running agent for the current working directory's
// space and returns an HTTP client pointed at it.
//
// The supervisor is the source of truth; if no agent is running for the
// current space, an error is returned.
func Current(ctx context.Context) (*agentclient.Client, api.RuntimeInfo, error) {
	name, err := spacemodel.CurrentName()
	if err != nil {
		return nil, api.RuntimeInfo{}, err
	}
	sup := supclient.New()
	rt, err := sup.RuntimeBySpace(ctx, name)
	if err != nil {
		if supclient.IsNotFound(err) {
			return nil, api.RuntimeInfo{}, fmt.Errorf("no runtime running for space %q — start it with 'quark run'", name)
		}
		return nil, api.RuntimeInfo{}, err
	}
	if rt.Status != api.RuntimeRunning {
		return nil, api.RuntimeInfo{}, fmt.Errorf("runtime for space %q is %s, not running", name, rt.Status)
	}
	return agentclient.New(rt.URL()), rt, nil
}
