package kb

type Entry struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     []byte `json:"value"`
}

type QueryRequest struct {
	SQL string `json:"sql"`
}

type QueryResponse struct {
	Rows []map[string]interface{} `json:"rows"`
}

type ListResponse struct {
	Entries []EntrySummary `json:"entries"`
}

type EntrySummary struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Size      int    `json:"size"`
}
