package main

import (
	"os"

	"github.com/fjukstad/gotoscapejs"
)

func main() {
	cy := &gotoscapejs.Cytoscape{}

	cy.Add(gotoscapejs.Element{
		Group: "nodes",
		Data: gotoscapejs.Data{
			Id: "a",
		},
	})

	cy.Add(gotoscapejs.Element{
		Group: "nodes",
		Data: gotoscapejs.Data{
			Id: "b",
		},
	})

	cy.Add(gotoscapejs.Element{
		Group: "nodes",
		Data: gotoscapejs.Data{
			Id: "c",
		},
	})

	cy.Add(gotoscapejs.Element{
		Group: "edges",
		Data: gotoscapejs.Data{
			Id:     "ab",
			Source: "a",
			Target: "b",
		},
	})

	cy.Add(gotoscapejs.Element{
		Group: "edges",
		Data: gotoscapejs.Data{
			Id:     "bc",
			Source: "b",
			Target: "c",
		},
	})

	cy.Add(gotoscapejs.Element{
		Group: "edges",
		Data: gotoscapejs.Data{
			Id:     "ca",
			Source: "c",
			Target: "a",
		},
	})

	cy.Write(os.Stdout)
}
