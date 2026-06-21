package model

import "time"

type NodeSummary struct {
        Name       string `json:"name"`
        SystemName string `json:"systemName"`
        Namespace  string `json:"namespace"`
        URI        string `json:"uri"`
        Category   string `json:"category"`
        State      string `json:"state"`
        Health     string `json:"health"`
        Version    int64  `json:"version"`
}

type NodeDetail struct {
        Name         string                 `json:"name"`
        SystemName   string                 `json:"systemName"`
        Namespace    string                 `json:"namespace"`
        URI          string                 `json:"uri"`
        Category     string                 `json:"category"`
        State        string                 `json:"state"`
        Health       string                 `json:"health"`
        Version      int64                  `json:"version"`
        ErrorMessage string                 `json:"errorMessage,omitempty"`
        CreatedAt    time.Time              `json:"createdAt"`
        UpdatedAt    time.Time              `json:"updatedAt"`
        Config       map[string]interface{} `json:"config"`
        Labels       map[string]string      `json:"labels"`
        Annotations  map[string]string      `json:"annotations"`
        Listens      []string               `json:"listens"`
        Events       []string               `json:"events"`
}
