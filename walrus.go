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

func run(c *client.Client, rootpath, filename string) error {
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

		mountpath := "/walrus/" + stage.Name
		hostpath := rootpath + "/" + stage.Name

		err = os.MkdirAll(hostpath, 06444)
		if err != nil {
			return err
		}

		fmt.Println(stage.Env, stage.Cmd, stage.Stdin)

		resp, err := c.ContainerCreate(context.Background(),
			&container.Config{Image: image,
				Env:          stage.Env,
				Cmd:          stage.Cmd,
				AttachStdin:  true,
				AttachStdout: true,
				AttachStderr: true,
				OpenStdin:    true,
				Tty:          true,
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
		// containerName := stage.Name + "-" + containerId[0:11]
		// err = c.ContainerRename(context.Background(), containerId, containerName)
		// if err != nil {
		// 	return err
		// }

		err = c.ContainerStart(context.Background(), containerId,
			types.ContainerStartOptions{})

		if err != nil {
			return err
		}

		hijackedResp, err := c.ContainerAttach(context.Background(), containerId, types.ContainerAttachOptions{
			Stream: true,
			Stdin:  true})
		if err != nil {
			return err
		}
		input := strings.Join(stage.Stdin, "\n")
		input = input + "\n"
		fmt.Println(input)
		_, err = hijackedResp.Conn.Write([]byte(input))
		if err != nil {
			return err
		}

	}

	return nil
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

	switch *cmd {
	case "run":
		err = run(client, hostpath, *configFilename)
	case "init":
		err = initWalrus(client, hostpath, mountpath)
	}

	fmt.Println(err)

}
