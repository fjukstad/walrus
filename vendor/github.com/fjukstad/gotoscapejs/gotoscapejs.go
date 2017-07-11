package gotoscapejs

import (
	"encoding/json"
	"io"
)

type Cytoscape struct {
	Elements []Element `json:"elements"`
}

type Element struct {
	Group    string   `json:"group,omitempty"`
	Data     Data     `json:"data,omitempty"`
	Position Position `json:"position,omitempty"`
}

type Position struct {
	X float64 `json:"x,omitempty"`
	Y float64 `json:"y,omitempty"`
}

type Data struct {
	Id     string                 `json:"id,omitempty"`
	Parent string                 `json:"parent,omitempty"`
	Source string                 `json:"source,omitempty"`
	Target string                 `json:"target,omitempty"`
	Data   map[string]interface{} `json:"data,omitempty"`
}

func (cy *Cytoscape) Add(e Element) {
	cy.Elements = append(cy.Elements, e)
}

func (cy *Cytoscape) Write(w io.Writer) {
	b, _ := json.Marshal(cy)
	w.Write(b)
}
