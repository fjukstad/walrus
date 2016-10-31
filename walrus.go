package main

import (
	"context"
	"encoding/json"
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
	// generate a version number/name if the pipeline description didn't
	if p.Version == "" {
		p.Version = createName()
	}

	for _, stage := range p.Stages {
		repo, tag := getRepoAndTag(stage.Image)
		image := repo + ":" + tag
		_, err := c.ImagePull(context.Background(), image, types.ImagePullOptions{})
		if err != nil {
			return err
		}

		mountpath := "/walrus/" + stage.Name
		hostpath := rootpath + "/" + stage.Name

		// Note the 0777 permission bits. We use such liberal bits since
		// we do not know about the users within the docker containers that
		// are going to be run. We want to fix this later!
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

	// Restore permission bits to output directory
	// err := filepath.Walk(rootpath, func(name string, info os.FileInfo, err error) error {
	// 	return os.Chmod(name, 0666)
	// })
	// if err != nil {
	// 	return err
	// }

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

// Saves the pipeline configuration (json) to a new .walrus directory in the
// output directory specified by the user. Can be used to determine what
// produced the output in the output directory.
func saveConfiguration(hostpath string, p *Pipeline) error {
	configPath := createConfigPath(hostpath)
	err := os.Mkdir(configPath, 0777)
	if err != nil {
		return err
	}

	filename := configPath + "/" + "pipeline.json"
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}

	_, err = f.Write(b)
	if err != nil {
		return err
	}

	return nil

}

// Moves the output of the previous runs into new folders for each stage. The
// names are STAGENAME-VERSION.
func savePreviousRun(hostpath string) error {

	// Check if there is any output from the previous runs
	f, err := os.Open(hostpath)
	if err != nil {
		// Output dir does not exist, nothing to back up.
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	files, err := f.Readdir(-1)
	if err != nil {
		return err
	}

	// No files in output directory, nothing to back up
	if len(files) == 0 {
		return nil
	}

	// Read old pipeline description to get it's version (use it for renaming)
	configPath := createConfigPath(hostpath)
	configFilename := configPath + "/" + "pipeline.json"
	p, err := ParseConfig(configFilename)
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(hostpath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			// Checks if directory name is a single word, i.e. a stage name that
			// has not yet been 'backed up' yet.
			if len(strings.Split(file.Name(), "-")) == 1 {
				oldFile := absPath + "/" + file.Name()
				newFile := absPath + "/" + file.Name() + "-" + p.Version
				err = os.Rename(oldFile, newFile)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Returns the full path of the  walrus configuration directory
func createConfigPath(hostpath string) string {
	return hostpath + "/" + ".walrus"
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
	err = savePreviousRun(hostpath)
	if err != nil {
		fmt.Println(err)
		return
	}

	switch *cmd {
	case "run":
		err = run(client, p, hostpath, *configFilename)
	case "init":
		err = initWalrus(client, hostpath, mountpath)
	}

	if err != nil {
		fmt.Println(err)
		return
	}

	err = saveConfiguration(hostpath, p)

	if err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println("Pipeline done. Have a look in ", hostpath, "for your results")
	}

}
