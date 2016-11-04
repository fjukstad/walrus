package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

var stageMutexes []*sync.Mutex
var completedConditions []*sync.Cond
var completedStages []bool

func run(c *client.Client, p *Pipeline, rootpath, filename string) error {
	// generate a version number/name if the pipeline description didn't
	if p.Version == "" {
		p.Version = createName()
	}

	stageMutexes = make([]*sync.Mutex, len(p.Stages))
	completedConditions = make([]*sync.Cond, len(p.Stages))
	completedStages = make([]bool, len(p.Stages))

	for i := range stageMutexes {
		stageMutexes[i] = &sync.Mutex{}
		completedConditions[i] = sync.NewCond(stageMutexes[i])
	}

	e := make(chan error, len(p.Stages))

	for i, stage := range p.Stages {
		go func(i int, stage *Stage) {
			repo, tag := getRepoAndTag(stage.Image)
			image := repo + ":" + tag
			_, err := c.ImagePull(context.Background(), image, types.ImagePullOptions{})
			if err != nil {
				e <- errors.Wrap(err, "Could not pull image")
			}

			mountpath := "/walrus/" + stage.Name
			hostpath := rootpath + "/" + stage.Name

			// Note the 0777 permission bits. We use such liberal bits since
			// we do not know about the users within the docker containers that
			// are going to be run. We want to fix this later!
			err = os.MkdirAll(hostpath, 0777)
			if err != nil {
				e <- errors.Wrap(err, "Could not create output directory for stage")
			}

			// If the stage has any inputs it waits for these stages to complete
			// before starting
			if len(stage.Inputs) > 0 {
				for j := range stage.Inputs {
					cond := completedConditions[j]
					cond.L.Lock()
					for !completedStages[j] {
						cond.Wait()
					}
					cond.L.Unlock()
				}
			}

			binds := []string{hostpath + ":" + mountpath}
			binds = append(binds, stage.Volumes...)

			resp, err := c.ContainerCreate(context.Background(),
				&container.Config{Image: image,
					Env: stage.Env,
					Cmd: stage.Cmd,
				},
				&container.HostConfig{
					Binds:       binds,
					VolumesFrom: stage.Inputs},
				&network.NetworkingConfig{},
				stage.Name)

			if err != nil {
				e <- errors.Wrap(err, "Could not create container")
			}
			containerId := resp.ID
			err = c.ContainerStart(context.Background(), containerId,
				types.ContainerStartOptions{})

			if err != nil {
				e <- errors.Wrap(err, "Could not start container")
			}

			_, err = c.ContainerWait(context.Background(), containerId)
			if err != nil {
				e <- errors.Wrap(err, "Failed to wait for container to finish")
			}

			cond := completedConditions[i]
			cond.L.Lock()
			// Notifies waiting stages on completion
			completedStages[i] = true
			cond.Broadcast()
			cond.L.Unlock()

			fmt.Println("Stage", stage.Name, "completed successfully")

			e <- nil
		}(i, stage)
	}

	for i := 0; i < len(p.Stages); i++ {
		err := <-e
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
func stopPreviousRun(c *client.Client, stages []*Stage) {
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
		return errors.Wrap(err, "Could not create directory to save old pipeline results")
	}

	filename := configPath + "/" + "pipeline.json"
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrap(err, "Could not open old pipeline configuration")
	}
	b, err := json.Marshal(p)
	if err != nil {
		return errors.Wrap(err, "Could not marshal pipeline configuration")
	}

	_, err = f.Write(b)
	if err != nil {
		return errors.Wrap(err, "Could not write pipeline configuration")
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

		return errors.Wrap(err, "Could not open previous pipeline outputs")
	}
	files, err := f.Readdir(-1)
	if err != nil {
		return errors.Wrap(err, "Could not read output directory")
	}

	if len(files) == 0 {
		return errors.Wrap(err, "No files in output directory, nothing to back up")
	}

	// Read old pipeline description to get it's version (use it for renaming)
	configPath := createConfigPath(hostpath)
	configFilename := configPath + "/" + "pipeline.json"
	p, err := ParseConfig(configFilename)
	if err != nil {
		return errors.Wrap(err, "Could not parse old pipeline configuration")
	}

	absPath, err := filepath.Abs(hostpath)
	if err != nil {
		return errors.Wrap(err, "Could not get the absolute path of the output directory")
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
					return errors.Wrap(err, "Could not rename stage output directory")
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

func fixMountPaths(stages []*Stage) error {
	for i, stage := range stages {
		updatedVolumes := []string{}
		for _, volume := range stage.Volumes {
			hostClientPath := strings.Split(volume, ":")

			if len(hostClientPath) > 2 {
				return errors.New("Incorrect volume " + volume + " in pipeline description")
			}

			hostPath := hostClientPath[0]
			clientPath := hostClientPath[1]

			if strings.HasPrefix(hostPath, "/") {
				updatedVolumes = append(updatedVolumes, volume)
				continue
			}

			absPath, err := filepath.Abs(hostPath)
			if err != nil {
				return errors.Wrap(err, "Could not get the absolute path of the mount path")
			}
			updatedVolumes = append(updatedVolumes, absPath+":"+clientPath)
		}
		stages[i].Volumes = updatedVolumes
	}

	return nil
}

func main() {
	var configFilename = flag.String("f", "pipeline.json", "pipeline description file")
	var outputDir = flag.String("output", "walrus", "where walrus should store output data on the host")

	flag.Parse()

	hostpath, err := filepath.Abs(*outputDir)
	if err != nil {
		fmt.Println("Check hostpath", err)
	}

	flag.Parse()
	client, err := client.NewEnvClient()
	if err != nil {
		fmt.Println(err)
		return
	}

	p, err := ParseConfig(*configFilename)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = fixMountPaths(p.Stages)
	if err != nil {
		fmt.Println(err)
		return
	}

	stopPreviousRun(client, p.Stages)
	err = savePreviousRun(hostpath)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = run(client, p, hostpath, *configFilename)

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
