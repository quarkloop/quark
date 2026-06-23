package model

type NamespaceSummary struct {
	Namespace     string `json:"namespace"`
	SystemCount   int    `json:"systemCount"`
	NodeCount     int    `json:"nodeCount"`
	HealthyNodes  int64  `json:"healthyNodes"`
	UnhealthyNodes int64 `json:"unhealthyNodes"`
}
