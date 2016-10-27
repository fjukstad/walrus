package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

func run(c *client.Client, filename string) error {
	p, err := ParseConfig(filename)
	if err != nil {
		return err
	}

	fmt.Println("Pulling docker images")

	for _, stage := range p.Stages {
		repo, tag := getRepoAndTag(stage.Image)
		image := repo + ":" + tag
		fmt.Print(repo, ":", tag, "\n")
		_, err := c.ImagePull(context.Background(), image, types.ImagePullOptions{})
		if err != nil {
			return err
		}

		resp, err := c.ContainerCreate(context.Background(),
			&container.Config{Image: image,
				Env: stage.Env,
				Cmd: stage.Cmd,
			},
			&container.HostConfig{},
			&network.NetworkingConfig{},
			stage.Name)
		if err != nil {
			return err
		}
		containerId := resp.ID
		containerName := stage.Name + "-" + containerId[0:11]
		err = c.ContainerRename(context.Background(), containerId, containerName)
		if err != nil {
			return err
		}

		err = c.ContainerStart(context.Background(), containerId, types.ContainerStartOptions{})
		if err != nil {
			return err
		}
		fmt.Println(containerId, err)

	}

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
	client, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	switch *cmd {
	case "run":
		err = run(client, *configFilename)
	}

	fmt.Println(err)

}
