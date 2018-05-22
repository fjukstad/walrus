package main

import (
	"fmt"
	"net/http"

	"github.com/fjukstad/gotoscapejs"
	"github.com/fjukstad/walrus/pipeline"
)

var vis = `
<head>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape/3.2.12/cytoscape.js"></script>
    <script
      src="https://code.jquery.com/jquery-3.1.1.js"
      integrity="sha256-16cdPddA6VdVInumRGo6IbivbERE8p7CQR3HzTBuELA="
      crossorigin="anonymous"></script>
    <script src="https://unpkg.com/dagre@0.7.4/dist/dagre.js"></script>
    <script src="https://cytoscape.github.io/cytoscape.js-dagre/cytoscape-dagre.js"></script> 
    <script src="http://cdnjs.cloudflare.com/ajax/libs/qtip2/2.2.0/jquery.qtip.min.js"></script>
    <link href="http://cdnjs.cloudflare.com/ajax/libs/qtip2/2.2.0/jquery.qtip.min.css" rel="stylesheet" type="text/css" />
    <script src="https://cdn.rawgit.com/cytoscape/cytoscape.js-qtip/2.7.1/cytoscape-qtip.js"></script>
    <style>
        #cy {
            width: 100%;
            height: 100%;
        }
    </style> 
</head>
<body>
    <div id="cy"></div> 
    <script>
    var cy = cytoscape({
      container: $('#cy'),
      boxSelectionEnabled: false,
      autounselectify: true,
	  layout: {
		  name:'dagre'
	  }, 
      style: [
      {
          selector: 'node',
          style: {
              'content': 'data(id)',
              'text-opacity': 0.5,
              'text-valign': 'center',
              'text-halign': 'right',
              'background-color': '#11479e'
          }
      },
      {
          selector: 'edge',
          style: {
              'curve-style': 'bezier',
              'width': 4,
              'target-arrow-shape': 'triangle',
              'line-color': '#9dbaea',
              'target-arrow-color': '#9dbaea',
          }
      }
      ],
    });
    $.get( "/graph", function( data ) {
        console.log(data) 
        cy.add(JSON.parse(data).elements)
		console.log(cy.nodes())
        cy.layout({
            name:"dagre"
        }).run();

        cy.elements().qtip({
            content: function(){
                d = this.data().data; 
                str = "" 
                for (var key in d) {
                    str += "<b>"+key+":</b></br>"
                    cont = [].concat(d[key]);
                    for(var i in cont){
                        str += cont[i] + "</br>"
                    }
                }

                return str
            },
            position: {
              my: 'right center',
              at: 'left center'
            },
            style: {
              classes: 'qtip-bootstrap',
              tip: {
                width: 16,
                height: 8
              }
            }
        })
    });
    </script> 
</body>
`

func startPipelineVisualization(p *pipeline.Pipeline, port string) error {
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
					"Comment": stage.Comment,
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
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte(vis))
	})

	fmt.Println("View pipeline visualization on http://localhost" + port)
	return http.ListenAndServe(port, mux)

}
