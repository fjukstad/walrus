package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

func run(c *docker.Client, filename string) error {
	p, err := ParseConfig(filename)
	if err != nil {
		return err
	}

	fmt.Println("Pulling docker images")

	for _, stage := range p.Stages {
		image := stage.Image
		repo, tag := getRepoAndTag(image)
		fmt.Print(repo, ":", tag, "\n")
		pom := docker.PullImageOptions{Repository: repo,
			Tag: tag}

		err = c.PullImage(pom, docker.AuthConfiguration{})
		if err != nil {
			return err
		}

	}

	fmt.Println(p)
	return nil
}

func getRepoAndTag(pipelineImage string) (repo, tag string) {
	repoAndTag := strings.Split(pipelineImage, ":")
	if len(repoAndTag) == 1 {
		tag = "latest"
	} else {
		tag = repoAndTag[1]
	}
	repo = repoAndTag[0]

	return repo, tag
}

func main() {
	var configFilename = flag.String("f", "pipeline.json", "pipeline description file")
	var cmd = flag.String("cmd", "run", "walrus command. available commands: 'run'")

	flag.Parse()

	client, err := docker.NewClientFromEnv()
	if err != nil {
		panic(err)
	}

	switch *cmd {
	case "run":
		err = run(client, *configFilename)
	}

	fmt.Println(err)

}
