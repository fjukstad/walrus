package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/fjukstad/gotoscapejs"
	"github.com/fjukstad/walrus/pipeline"
)

func main() {

	var pipelineFilename = flag.String("p", "pipeline.json", "pipeline definition. see github.com/fjukstad/walrus for more info")

	flag.Parse()

	p, err := pipeline.ParseConfig(*pipelineFilename)
	if err != nil {
		fmt.Println(err)
		return
	}

	cy := &gotoscapejs.Cytoscape{}

	for _, stage := range p.Stages {
		cy.Add(gotoscapejs.Element{
			Group: "nodes",
			Data: gotoscapejs.Data{
				Id: stage.Name,
				Data: map[string]interface{}{
					"Image":   stage.Image,
					"Cmd":     stage.Cmd,
					"Env":     stage.Env,
					"Volumes": stage.Volumes,
					"Inputs":  stage.Inputs,
				},
			},
		})

		for _, input := range stage.Inputs {
			cy.Add(gotoscapejs.Element{
				Group: "edges",
				Data: gotoscapejs.Data{
					Id:     stage.Name + input,
					Source: input,
					Target: stage.Name,
				},
			})
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/graph", func(w http.ResponseWriter, req *http.Request) {
		cy.Write(w)
	})
	mux.Handle("/", http.FileServer(http.Dir(".")))

	fmt.Println("Demo running on localhost:9090")
	err = http.ListenAndServe(":9090", mux)
	fmt.Println(err)

}
