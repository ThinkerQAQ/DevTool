package merman

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	diagram "github.com/thinkerqaq/devtool/sdk/diagram"
)

const maxGraphNodes = 160
const maxGraphEdges = 320

type model struct {
	Type  string `json:"type"`
	Nodes []struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"nodes"`
	Edges []struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Start string `json:"start"`
		End   string `json:"end"`
		Label string `json:"label"`
	} `json:"edges"`
	Subgraphs []struct {
		ID    string   `json:"id"`
		Title string   `json:"title"`
		Nodes []string `json:"nodes"`
	} `json:"subgraphs"`
	Actors map[string]struct {
		Description string `json:"description"`
	} `json:"actors"`
	ActorOrder []string `json:"actorOrder"`
	Messages   []struct {
		From    *string `json:"from"`
		To      *string `json:"to"`
		Message string  `json:"message"`
		Type    int     `json:"type"`
	} `json:"messages"`
}

func (e *Extension) semantic(ctx context.Context, bin, source string) (diagram.Graph, error) {
	g := diagram.Graph{Nodes: []diagram.GraphNode{}, Edges: []diagram.GraphEdge{}, Groups: []diagram.GraphGroup{}}
	if len(source) == 0 {
		return g, fmt.Errorf("empty Mermaid source")
	}
	run, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	cmd := exec.CommandContext(run, bin, "parse", "--resource-profile", "constrained", "-")
	cmd.Stdin = strings.NewReader(source)
	raw, err := cmd.Output()
	if err != nil {
		if run.Err() != nil {
			return g, fmt.Errorf("Merman parse interrupted: %w", run.Err())
		}
		return g, fmt.Errorf("Merman semantic parse: %w", err)
	}
	if len(raw) > 6<<20 {
		return g, fmt.Errorf("Merman semantic output exceeds 6 MiB")
	}
	var data model
	if err := json.Unmarshal(raw, &data); err != nil {
		return g, fmt.Errorf("decode Mermaid semantic model: %w", err)
	}
	g.Kind = data.Type
	switch data.Type {
	case "flowchart-v2", "flowchart", "stateDiagram":
		g.NodeCount = len(data.Nodes)
		g.EdgeCount = len(data.Edges)
		for i, n := range data.Nodes {
			if i >= maxGraphNodes {
				g.Truncated = true
				break
			}
			g.Nodes = append(g.Nodes, diagram.GraphNode{ID: n.ID, Label: n.Label})
		}
		for i, edge := range data.Edges {
			if i >= maxGraphEdges {
				g.Truncated = true
				break
			}
			from, to := edge.From, edge.To
			if from == "" {
				from = edge.Start
			}
			if to == "" {
				to = edge.End
			}
			g.Edges = append(g.Edges, diagram.GraphEdge{From: from, To: to, Label: edge.Label})
		}
		for i, group := range data.Subgraphs {
			if i >= 32 {
				g.Truncated = true
				break
			}
			g.Groups = append(g.Groups, diagram.GraphGroup{ID: group.ID, Label: group.Title, Nodes: group.Nodes})
		}
	case "sequence":
		g.NodeCount = len(data.ActorOrder)
		for i, id := range data.ActorOrder {
			if i >= maxGraphNodes {
				g.Truncated = true
				break
			}
			g.Nodes = append(g.Nodes, diagram.GraphNode{ID: id, Label: data.Actors[id].Description})
		}
		for _, msg := range data.Messages {
			if msg.From == nil || msg.To == nil || *msg.From == "" || *msg.To == "" {
				continue
			}
			g.EdgeCount++
			if len(g.Edges) >= maxGraphEdges {
				g.Truncated = true
				continue
			}
			g.Edges = append(g.Edges, diagram.GraphEdge{From: *msg.From, To: *msg.To, Label: msg.Message})
		}
	default:
		return g, fmt.Errorf("Merman semantic graph type %q unsupported", data.Type)
	}
	groupOf := map[string]string{}
	for _, gr := range g.Groups {
		for _, id := range gr.Nodes {
			groupOf[id] = gr.ID
		}
	}
	for _, edge := range g.Edges {
		a, b := groupOf[edge.From], groupOf[edge.To]
		if a != "" && b != "" && a != b {
			g.CrossLayerEdges++
			if strings.TrimSpace(edge.Label) == "" {
				g.UnlabeledCrossLayerEdges++
			}
		}
	}
	return g, nil
}
