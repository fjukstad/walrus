package pipeline

import (
	"hash/fnv"
	"io/ioutil"

	"github.com/pkg/errors"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
)

type Node struct {
	Name string
}

func (n Node) ID() int64 {
	h := fnv.New32a()
	h.Write([]byte(n.Name))
	return int64(h.Sum32())
}

func (n Node) DOTID() string {
	return n.Name
}

type Edge struct {
	F  Node
	T  Node
	id int64
}

func (e Edge) From() graph.Node {
	return e.F
}

func (e Edge) To() graph.Node {
	return e.T
}

func (e Edge) ID() int64 {
	return e.id
}

func (p *Pipeline) WriteDOT(filename string) error {

	graph := p.createGraph()

	b, err := dot.Marshal(graph, p.Name, "", "", false)
	if err != nil {
		return errors.Wrap(err, "Could not marshal graph")
	}

	err = ioutil.WriteFile(filename, b, 0644)
	if err != nil {
		return errors.Wrap(err, "Could not write dot file")
	}

	return nil

}

func (p *Pipeline) createGraph() graph.Directed {
	graph := simple.NewDirectedGraph()

	for _, stage := range p.Stages {
		for _, input := range stage.Inputs {
			t := Node{stage.Name}
			f := Node{input}
			e := Edge{f, t, 0}
			graph.SetEdge(e)
		}
	}
	return graph
}
