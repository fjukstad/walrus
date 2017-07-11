package main

import (
	"fmt"
	"net/http"

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

	mux := http.NewServeMux()
	mux.HandleFunc("/graph", func(w http.ResponseWriter, req *http.Request) {
		cy.Write(w)
	})
	mux.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("Demo running on localhost:9090")
	err := http.ListenAndServe(":9090", mux)
	fmt.Println(err)

}
