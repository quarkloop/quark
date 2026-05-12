package dgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/quarkloop/services/indexer/pkg/indexer"
)

func (d *Driver) GetNeighborhood(ctx context.Context, nodeID string, depth int) (*indexer.GraphFragment, error) {
	if depth <= 0 {
		depth = 1
	}

	nodes := map[string]indexer.GraphNode{}
	edges := map[string]indexer.GraphEdge{}
	frontier := []string{nodeID}
	for level := 0; level < depth && len(frontier) > 0; level++ {
		next, err := d.expandNeighborhood(ctx, frontier, nodes, edges)
		if err != nil {
			return nil, err
		}
		frontier = next
	}
	return sortedGraphFragment(nodes, edges), nil
}

func (d *Driver) expandNeighborhood(ctx context.Context, ids []string, nodes map[string]indexer.GraphNode, edges map[string]indexer.GraphEdge) ([]string, error) {
	nextSet := map[string]bool{}
	for _, id := range ids {
		resp, err := d.client.NewReadOnlyTxn().QueryWithVars(ctx, neighborhoodQuery, map[string]string{"$id": id})
		if err != nil {
			return nil, fmt.Errorf("neighborhood %s: %w", id, err)
		}
		var payload neighborhoodPayload
		if err := json.Unmarshal(resp.GetJson(), &payload); err != nil {
			return nil, fmt.Errorf("decode neighborhood: %w", err)
		}
		payload.mergeInto(nodes, edges, nextSet)
	}

	next := make([]string, 0, len(nextSet))
	for id := range nextSet {
		if id != "" {
			next = append(next, id)
		}
	}
	sort.Strings(next)
	return next, nil
}

func sortedGraphFragment(nodes map[string]indexer.GraphNode, edges map[string]indexer.GraphEdge) *indexer.GraphFragment {
	out := &indexer.GraphFragment{
		Nodes: make([]indexer.GraphNode, 0, len(nodes)),
		Edges: make([]indexer.GraphEdge, 0, len(edges)),
	}
	for _, node := range nodes {
		out.Nodes = append(out.Nodes, node)
	}
	for _, edge := range edges {
		out.Edges = append(out.Edges, edge)
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	sort.Slice(out.Edges, func(i, j int) bool {
		return out.Edges[i].FromID+"|"+out.Edges[i].Relation+"|"+out.Edges[i].ToID <
			out.Edges[j].FromID+"|"+out.Edges[j].Relation+"|"+out.Edges[j].ToID
	})
	return out
}

type neighborhoodPayload struct {
	Chunks []struct {
		Entities []dgraphEntity `json:"quark.chunk_entity"`
	} `json:"chunks"`
	Entities []dgraphNeighborhoodEntity `json:"entities"`
}

func (p neighborhoodPayload) mergeInto(nodes map[string]indexer.GraphNode, edges map[string]indexer.GraphEdge, nextSet map[string]bool) {
	for _, chunk := range p.Chunks {
		for _, entity := range chunk.Entities {
			addNode(nodes, entity)
			nextSet[entity.ID] = true
		}
	}
	for _, entity := range p.Entities {
		self := entity.asEntity()
		addNode(nodes, self)
		for _, rel := range entity.Out {
			for _, to := range rel.To {
				addNode(nodes, to)
				key := self.ID + "|" + rel.Name + "|" + to.ID
				edges[key] = indexer.GraphEdge{FromID: self.ID, ToID: to.ID, Relation: rel.Name}
				nextSet[to.ID] = true
			}
		}
		for _, rel := range entity.In {
			for _, from := range rel.From {
				addNode(nodes, from)
				key := from.ID + "|" + rel.Name + "|" + self.ID
				edges[key] = indexer.GraphEdge{FromID: from.ID, ToID: self.ID, Relation: rel.Name}
				nextSet[from.ID] = true
			}
		}
	}
}

type dgraphEntity struct {
	ID   string `json:"quark.entity_id"`
	Name string `json:"quark.entity_name"`
	Type string `json:"quark.entity_type"`
}

type dgraphNeighborhoodEntity struct {
	ID   string              `json:"quark.entity_id"`
	Name string              `json:"quark.entity_name"`
	Type string              `json:"quark.entity_type"`
	Out  []dgraphOutRelation `json:"out"`
	In   []dgraphInRelation  `json:"in"`
}

type dgraphOutRelation struct {
	Name string           `json:"quark.relation_name"`
	To   dgraphEntityList `json:"quark.relation_to"`
}

type dgraphInRelation struct {
	Name string           `json:"quark.relation_name"`
	From dgraphEntityList `json:"quark.relation_from"`
}

type dgraphEntityList []dgraphEntity

func (l *dgraphEntityList) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) || len(data) == 0 {
		return nil
	}
	if data[0] == '[' {
		var entities []dgraphEntity
		if err := json.Unmarshal(data, &entities); err != nil {
			return err
		}
		*l = entities
		return nil
	}
	var entity dgraphEntity
	if err := json.Unmarshal(data, &entity); err != nil {
		return err
	}
	*l = []dgraphEntity{entity}
	return nil
}

func (e dgraphNeighborhoodEntity) asEntity() dgraphEntity {
	return dgraphEntity{ID: e.ID, Name: e.Name, Type: e.Type}
}

func addNode(nodes map[string]indexer.GraphNode, entity dgraphEntity) {
	if entity.ID == "" {
		return
	}
	nodes[entity.ID] = indexer.GraphNode{ID: entity.ID, Label: entity.Name, Type: entity.Type}
}
