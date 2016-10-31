package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

func run(c *client.Client, p *Pipeline, rootpath, filename string) error {
	for _, stage := range p.Stages {
		repo, tag := getRepoAndTag(stage.Image)
		image := repo + ":" + tag
		_, err := c.ImagePull(context.Background(), image, types.ImagePullOptions{})
		if err != nil {
			return err
		}

		mountpath := "/walrus/" + stage.Name
		hostpath := rootpath + "/" + stage.Name
		err = os.MkdirAll(hostpath, 0777)
		if err != nil {
			return err
		}

		resp, err := c.ContainerCreate(context.Background(),
			&container.Config{Image: image,
				Env: stage.Env,
				Cmd: stage.Cmd,
			},
			&container.HostConfig{
				Binds:       []string{hostpath + ":" + mountpath},
				VolumesFrom: stage.Inputs},
			&network.NetworkingConfig{},
			stage.Name)

		if err != nil {
			return err
		}
		containerId := resp.ID
		err = c.ContainerStart(context.Background(), containerId,
			types.ContainerStartOptions{})

		if err != nil {
			return err
		}

		_, err = c.ContainerWait(context.Background(), containerId)
		if err != nil {
			return err
		}
	}
	return nil
}

// Stops any previously run pipeline and deletes the containers.
func stopPreviousRun(c *client.Client, stages []Stage) {
	for _, stage := range stages {
		c.ContainerStop(context.Background(), stage.Name, nil)
		c.ContainerRemove(context.Background(), stage.Name, types.ContainerRemoveOptions{})
	}
}

// Generate a list of volume mounts on the form
// /hostpath/stagename-containerid/:/walrus/stagename
func getInputVolumes(inputs []string, hostpath string) (volumes []string) {
	for _, input := range inputs {
		volumes = append(volumes, hostpath+"/"+input+":"+"/walrus"+"/"+input)
	}
	fmt.Println("VOLUMES", volumes)
	return volumes
}

func initWalrus(c *client.Client, hostpath, mountpath string) error {
	fmt.Println(hostpath, mountpath)
	volume := hostpath + ":" + mountpath
	_, err := c.ContainerCreate(context.Background(),
		&container.Config{Image: "ubuntu:14.04"},
		&container.HostConfig{Binds: []string{volume}},
		&network.NetworkingConfig{},
		"walrus")
	if err != nil {
		return err
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
	var outputDir = flag.String("output", ".", "where walrus should store output data on the host")

	flag.Parse()
	var mountpath = "/walrus"

	hostpath, err := filepath.Abs(*outputDir)
	if err != nil {
		fmt.Println("Check hostpath", err)
	}

	flag.Parse()
	client, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	p, err := ParseConfig(*configFilename)
	if err != nil {
		panic(err)
	}

	stopPreviousRun(client, p.Stages)

	switch *cmd {
	case "run":
		err = run(client, p, hostpath, *configFilename)
	case "init":
		err = initWalrus(client, hostpath, mountpath)
	}

	if err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println("Pipeline done. Have a look in ", hostpath, "for your results")
	}

}
