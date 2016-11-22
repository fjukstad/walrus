package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fjukstad/walrus/pipeline"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

var stageMutexes []*sync.Mutex
var completedConditions []*sync.Cond
var completedStages []bool
var stageIndex map[string]int

func run(c *client.Client, p *pipeline.Pipeline, rootpath, filename string) error {
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

	stageIndex = make(map[string]int, len(p.Stages))

	// Name to index mapping
	for i, stage := range p.Stages {
		stageIndex[stage.Name] = i
	}

	e := make(chan error, len(p.Stages))

	for i, stage := range p.Stages {
		go func(i int, stage *pipeline.Stage) {
			mountpath := "/walrus/" + stage.Name
			hostpath := rootpath + "/" + stage.Name

			repo, tag := getRepoAndTag(stage.Image)
			image := repo + ":" + tag
			rc, err := c.ImagePull(context.Background(), image,
				types.ImagePullOptions{})
			if err != nil {
				e <- errors.Wrap(err, "Could not pull image")
				return
			}

			defer rc.Close()

			_, err = ioutil.ReadAll(rc)
			if err != nil {
				e <- errors.Wrap(err, "error reading image pull")
			}

			// If the stage has any inputs it waits for these stages to complete
			// before starting
			if len(stage.Inputs) > 0 {
				for _, input := range stage.Inputs {
					index := stageIndex[input]
					cond := completedConditions[index]
					cond.L.Lock()
					for !completedStages[index] {
						cond.Wait()
					}
					cond.L.Unlock()
				}
			}

			// try to open output directory, if it exists then we can serve the
			// "cached"/old results
			_, err = os.Open(hostpath)

			if !stage.Cache || err != nil {

				// first remove any previous container (note that we're ignoring
				// errors)
				c.ContainerRemove(context.Background(), stage.Name, types.ContainerRemoveOptions{})

				// Note the 0777 permission bits. We use such liberal bits since
				// we do not know about the users within the docker containers that
				// are going to be run. We want to fix this later!
				err = os.MkdirAll(hostpath, 0777)
				if err != nil {
					e <- errors.Wrap(err, "Could not create output directory for stage")
					return
				}

				binds := []string{hostpath + ":" + mountpath}
				binds = append(binds, stage.Volumes...)

				resp, err := c.ContainerCreate(context.Background(),
					&container.Config{Image: image,
						Env:        stage.Env,
						Cmd:        stage.Cmd,
						Entrypoint: stage.Entrypoint,
					},
					&container.HostConfig{
						Binds:       binds,
						VolumesFrom: stage.Inputs},
					&network.NetworkingConfig{},
					stage.Name)

				if err != nil {
					e <- errors.Wrap(err, "Could not create container "+stage.Name)
					return
				}
				containerId := resp.ID

				err = c.ContainerStart(context.Background(), containerId,
					types.ContainerStartOptions{})

				if err != nil {
					e <- errors.Wrap(err, "Could not start container "+stage.Name)
					return
				}

				_, err = c.ContainerWait(context.Background(), containerId)
				if err != nil {
					e <- errors.Wrap(err, "Failed to wait for container to finish")
					return
				}

			}

			cond := completedConditions[i]
			cond.L.Lock()

			// Notifies waiting stages on completion
			completedStages[i] = true

			cond.Broadcast()
			cond.L.Unlock()

			exitCode, errmsg, err := exitCode(c, stage.Name)
			if err != nil {
				e <- err
			}

			logs, err := getLogs(c, stage.Name)
			if err != nil {
				e <- err
				return
			}

			err = writeLogs(logs, hostpath)
			if err != nil {
				e <- err
				return
			}

			if exitCode != 0 {
				e <- errors.New(stage.Name + " failed with exit code " + strconv.Itoa(exitCode) + "\n" + errmsg + "\n" + logs)
				return
			}
			fmt.Println(stage.Name, "completed successfully.")

			e <- nil
		}(i, stage)
	}
	var err error
	for i := 0; i < len(p.Stages); i++ {
		err = <-e
		if err != nil {
			fmt.Println(err)
		}
	}

	// Restore permission bits to output directory
	// err := filepath.Walk(rootpath, func(name string, info os.FileInfo, err error) error {
	// 	return os.Chmod(name, 0666)
	// })
	// if err != nil {
	// 	return err
	// }

	return err
}

func writeLogs(logs, path string) error {
	filename := path + "/walrus.log"
	return ioutil.WriteFile(filename, []byte(logs), 06444)
}

func exitCode(c *client.Client, container string) (int, string, error) {
	info, err := c.ContainerInspect(context.Background(), container)
	if err != nil {
		return 1, "", err
	}
	state := info.State
	return state.ExitCode, state.Error, nil
}

func getLogs(c *client.Client, container string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, _ := client.NewEnvClient()
	reader, err := client.ContainerLogs(ctx, container, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
	})
	if err != nil {
		return "", err
	}

	b, err := ioutil.ReadAll(reader)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(b), nil
}

// Stops any previously run pipeline and deletes the containers. If any, we
// ignore errors from docker.
func stopPreviousRun(c *client.Client, stages []*pipeline.Stage) {
	for _, stage := range stages {
		c.ContainerKill(context.Background(), stage.Name, "9")
	}
}

// Generate a list of volume mounts on the form
// /hostpath/stagename-containerid/:/walrus/stagename
func getInputVolumes(inputs []string, hostpath string) (volumes []string) {
	for _, input := range inputs {
		volumes = append(volumes, hostpath+"/"+input+":"+"/walrus"+"/"+input)
	}
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
func saveConfiguration(hostpath string, p *pipeline.Pipeline) error {
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
	p, err := pipeline.ParseConfig(configFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return errors.Wrap(err, "Could not parse old pipeline configuration")
	}

	absPath, err := filepath.Abs(hostpath)
	if err != nil {
		return errors.Wrap(err, "Could not get the absolute path of the output directory")
	}

	// iterate over all stages and move each output folder to a new directory
	// if the stage is cached the directory is copied. .
	for _, stage := range p.Stages {
		newFilename := absPath + "/" + stage.Name + "-" + p.Version
		oldFilename := absPath + "/" + stage.Name

		if stage.Cache {
			err = copyDir(oldFilename, newFilename)
		} else {
			err = os.Rename(oldFilename, newFilename)
		}
		if err != nil {
			return errors.Wrap(err, "Could not back up pipeline stage directory")
		}
	}

	// back up the walrus directory as well
	walrusDirectory := configPath
	newWalrusDirectory := walrusDirectory + "-" + p.Version
	err = os.Rename(walrusDirectory, newWalrusDirectory)
	if err != nil {
		return err
	}

	return nil
}

// Copies one directory to another. NOTE, only one level, not recursive yet.
func copyDir(src, dest string) error {

	err := os.Mkdir(dest, 0777)
	if err != nil {
		return err
	}
	err = filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		// nothing to back up
		if err != nil {
			return err
		}

		// if dir then create dir , else a file then copy it
		if info.IsDir() {
		} else {
			srcFile, err := os.Open(path)
			if err != nil {
				return err
			}

			destFilename := dest + "/" + info.Name()
			destFile, err := os.OpenFile(destFilename, os.O_CREATE, 0664)
			if err != nil {
				return err
			}

			_, err = io.Copy(srcFile, destFile)
			if err != nil {
				return err
			}

		}
		return nil
	})
	return err
}

// Returns the full path of the  walrus configuration directory
func createConfigPath(hostpath string) string {
	return hostpath + "/" + ".walrus"
}

func fixMountPaths(stages []*pipeline.Stage) error {
	for i, stage := range stages {
		updatedVolumes := []string{}
		for _, volume := range stage.Volumes {
			hostClientPath := strings.Split(volume, ":")

			if len(hostClientPath) > 2 {
				return errors.New("Incorrect volume " + volume + " in pipeline description")
			}

			hostPath := hostClientPath[0]

			var clientPath string
			if len(hostClientPath) < 2 {
				clientPath = hostPath
			} else {
				clientPath = hostClientPath[1]
			}

			if strings.HasPrefix(hostPath, "/") {
				updatedVolumes = append(updatedVolumes, volume)
				continue
			}

			absPath, err := filepath.Abs(hostPath)
			if err != nil {
				return errors.Wrap(err, "Could not get the absolute path of the mount path")
			}

			mount := absPath + ":" + clientPath
			if stage.MountPropagation != "" {
				mount = mount + ":" + stage.MountPropagation
			}

			updatedVolumes = append(updatedVolumes, mount)
		}
		stages[i].Volumes = updatedVolumes
	}

	return nil
}

func main() {
	var configFilename = flag.String("f", "pipeline.json", "pipeline description file")
	var outputDir = flag.String("output", "walrus", "where walrus should store output data on the host")
	var web = flag.Bool("web", false, "host interactive visualization of the pipeline")
	var port = flag.String("port", ":9090", "port to run web server for pipeline visualization")

	flag.Parse()

	// set umask to 000 while walrus is running (we want to have full read/write
	// permissions to the output dirs while running.
	oldmask := syscall.Umask(000)
	defer syscall.Umask(oldmask)

	hostpath, err := filepath.Abs(*outputDir)
	if err != nil {
		fmt.Println("Check hostpath", err)
		return
	}

	flag.Parse()
	client, err := client.NewEnvClient()
	if err != nil {
		fmt.Println(err)
		return
	}

	p, err := pipeline.ParseConfig(*configFilename)
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
	}

	if *web {
		go func() {
			err = startPipelineVisualization(p, *port)
			if err != nil {
				fmt.Println("Could not start pipeline visualization:", err)
			}
		}()
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
		fmt.Println("All stages completed successfully.",
			"\nOutput written to ", hostpath)
	}

}
