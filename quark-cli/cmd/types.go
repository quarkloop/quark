package cmd

type EventListOptions struct {
	Namespace     string
	System        string
	Node          string
	Kinds         string
	Since         string
	Until         string
	Limit         int
	AllNamespaces bool
}
