package client

import agentclient "github.com/quarkloop/agent-client"

type Client = agentclient.Transport
type ClientOption = agentclient.TransportOption

var NewClient = agentclient.NewTransport
