package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
)

func main() {
	var configFilename = flag.String("f", "pipeline.json", "pipeline description file")
	flag.Parse()

	p := Pipeline{}
	file, e := ioutil.ReadFile(*configFilename)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		return
	}
	fmt.Printf("%s\n", string(file))

	err := json.Unmarshal(file, &p)
	fmt.Println("Results", p, err)

	cli, err := client.NewEnvClient()
    if err != nil {
        panic(err)
    }

    containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
    if err != nil {
        panic(err)
    }

    for _, container := range containers {
        fmt.Printf("%s %s\n", container.ID[:10], container.Image)
    }

}
