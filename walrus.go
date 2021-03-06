package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	wcontainer "github.com/fjukstad/walrus/container"
	"github.com/fjukstad/walrus/lfs"
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
var currentUser string
var profile *bool

var numParallelWorkers = 5

func run(c *client.Client, p *pipeline.Pipeline, rootpath, filename string) error {

	// We use a buffered channel to limit the number of stages that can run in
	// parallel. Every stage will signal that it starts doing work by inserting
	// a 1 into the channel. Once it completes it will pull one number out of
	// the channel.
	executing := make(chan int, numParallelWorkers)

	stageMutexes = make([]*sync.Mutex, len(p.Stages))
	completedConditions = make([]*sync.Cond, len(p.Stages))
	completedStages = make([]bool, len(p.Stages))

	pipelineStart := time.Now()
	defer func() {
		p.Runtime = time.Since(pipelineStart)
	}()

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

			// Even if might be a parallel stage we only use the first part of
			// the name
			stageName := strings.Split(stage.Name, "_")[0]

			mountpath := "/walrus/" + stageName
			hostpath := rootpath + "/" + stageName

			repo, tag := getRepoAndTag(stage.Image)
			image := repo + ":" + tag

			images, err := c.ImageList(context.Background(), types.ImageListOptions{})
			if err != nil {
				e <- errors.Wrap(err, "Could not list images")
				return
			}

			// Check if Docker image is present on the host. If not pull it
			// down.
			imagePresent := false
			for _, img := range images {
				for _, tag := range img.RepoTags {
					if image == tag {
						imagePresent = true
					}
				}
			}
			if !imagePresent {
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
			}

			// If the stage has any inputs it waits for these stages to complete
			// before starting.
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

			// Requesting 'ticket' in workerpool.
			executing <- 1

			// If the stage can be cached, check for a previous run. If this
			// container does not exist we need to run the stage again. Also if
			// a cached stage has failed we'll need to re run it.
			if stage.Cache {
				code, _, err := exitCode(c, stage.Name)
				if err != nil {
					log.Println(err)
					log.Println("Warning: Could not find cached container", stage.Name, "will re-run the stage")
					stage.Cache = false
				}
				if code != 0 {
					stage.Cache = false
				}
			}

			// try to open output directory, if it exists then we can serve the
			// "cached"/old results
			_, err = os.Open(hostpath)

			if !stage.Cache || err != nil {
				// Removes a container with the same name as the stage.
				// This container could have been a previous run that the user
				// does not wish to cache, or a cached container which output
				// directory has been deleted. We ignore any error message
				// thrown9.
				c.ContainerRemove(context.Background(), stage.Name,
					types.ContainerRemoveOptions{RemoveVolumes: true,
						Force: true})

				// Note the 0777 permission bits. We use such liberal bits since
				// we do not know about the users within the docker containers
				// that are going to be run. We want to fix this later!
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
						User:       currentUser,
					},
					&container.HostConfig{
						Binds:       binds,
						VolumesFrom: stage.Inputs},
					&network.NetworkingConfig{},
					stage.Name)

				if err != nil || resp.ID == " " {
					e <- errors.Wrap(err, "Could not create container "+stage.Name)
					return
				}
				containerId := resp.ID

				stageStart := time.Now()

				numTries := 0

				if *profile {
					go wcontainer.Profile(c, containerId, hostpath+"/profile-"+stage.Name+".json")
				}

				for {
					err = c.ContainerStart(context.Background(), containerId,
						types.ContainerStartOptions{})
					if err != nil {
						log.Println("Warning: Could not start container", stage.Name, "retrying. Error:", err)
						if numTries > 10 {
							e <- errors.Wrap(err, "Could not start container "+stage.Name)
							return
						}
						numTries += 1
						time.Sleep(10 * time.Second)
					} else {
						break
					}
				}

				okC, errC := c.ContainerWait(context.Background(), containerId,
					container.WaitConditionNotRunning)
				select {
				case err := <-errC:
					if err != nil {
						e <- errors.Wrap(err, "Failed to wait for container to finish")
						return
					}
				case <-okC: // simply drain wait ok channel
				}

				stage.Runtime = time.Since(stageStart)

			}

			// Done executing, release ticket in worker pool.
			<-executing

			cond := completedConditions[i]
			cond.L.Lock()

			// Notifies waiting stages on completion
			completedStages[i] = true

			cond.L.Unlock()
			cond.Broadcast()

			exitCode, errmsg, err := exitCode(c, stage.Name)
			if err != nil {
				e <- errors.Wrap(err, "Could not get exit code for stage "+stage.Name)
			}

			logs, err := getLogs(c, stage.Name)
			if err != nil {
				e <- err
				return
			}

			err = writeLogs(logs, hostpath)
			if err != nil {
				e <- errors.Wrap(err, "Could not write logs for stage "+stage.Name)
				return
			}

			if exitCode != 0 {
				e <- errors.New("ERROR: Stage " + stage.Name + " failed with exit code " + strconv.Itoa(exitCode) + "\n" + stage.String() + "\n" + errmsg + "\n" + logs)
				return
			}

			log.Println("Stage", stage.Name, "completed successfully in", stage.Runtime)

			e <- nil
		}(i, stage)
	}

	var err error

	// Check for any error and return
	for range p.Stages {
		err = <-e
		if err != nil {
			return err
		}
	}

	// If version control is enabled we'll add all output data to the
	// repository and write commitid of the last stage to the pipeline
	// description file.
	if p.Commit {

		// Commit output data
		for i, stage := range p.Stages {

			// use first part of name since it might be a parallel stage with
			// the additional _paralell_stageName
			stageName := strings.Split(stage.Name, "_")[0]
			hostpath := rootpath + "/" + stageName

			// add and commit output data
			msg := "Add data pipeline stage: " + stageName
			commitId, err := lfs.AddAndCommitData(hostpath, msg)
			if err != nil {
				return errors.Wrap(err, "Could not commit output data "+stageName)
			}

			p.Stages[i].Version = commitId

			head, err := lfs.GetHead(hostpath)
			if err != nil {
				return errors.Wrap(err, "Could not get git head")
			}

			p.Version = head
		}

	}
	return nil
}

func writeLogs(logs, path string) error {
	filename := path + "/walrus.log"
	return ioutil.WriteFile(filename, []byte(logs), 0777)
}

func exitCode(c *client.Client, container string) (int, string, error) {
	info, err := c.ContainerInspect(context.Background(), container)
	if err != nil {
		return 0, "", err
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

// Stops any previously run pipeline and deletes the containers.
// Todo investigate if the docker pkg has defined some errors so that we don't
// have to do any string comparisons (ugly af).
func stopPreviousRun(c *client.Client, stages []*pipeline.Stage) error {
	for _, stage := range stages {
		err := c.ContainerKill(context.Background(), stage.Name, "9")
		if err != nil {
			if !strings.Contains(err.Error(), "No such") &&
				!strings.Contains(err.Error(), "not running") {
				return errors.Wrap(err, "Could not kill container "+stage.Name)
			}
		}

		// if the stage has caching enabled we can't remove it just yet. Its
		// exits codes etc. may be used in later pipeline runs
		if !stage.Cache {
			err = c.ContainerRemove(context.Background(), stage.Name,
				types.ContainerRemoveOptions{RemoveVolumes: true,
					Force: true})
			if err != nil {
				if !strings.Contains(err.Error(), "No such") {
					return errors.Wrap(err, "Could not remove container "+stage.Name)
				}
			}
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
			} else {

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
		}
		stages[i].Volumes = updatedVolumes
	}

	return nil
}

func main() {
	var configFilename = flag.String("i", "",
		"pipeline description file")
	var outputDir = flag.String("o", "walrus",
		"where walrus should store output data on the host")
	var web = flag.Bool("web", false,
		"host interactive visualization of the pipeline")
	var port = flag.String("p", ":9090",
		"port to run web server for pipeline visualization")
	var lfsServer = flag.Bool("lfs-server", false,
		"start an lfs-server, will not run the pipeline")
	var lfsServerDir = flag.String("lfs-server-dir", "lfs",
		"host directory to store lfs objects")
	var commit = flag.Bool("commit", false, "add and commit output data")
	var logs = flag.String("logs", "", "get logs for pipeline stage")
	var graphFilename = flag.String("graph", "", "write dot graph of the pipeline to the given filename and stop.")
	var printPipeline = flag.Bool("print", false, "print a pipeline configuration or completed pipeline \n\tconfiguration (use -i to specify its name and location)")
	var diff = flag.String("diff", "", "print difference of current pipeline run and the given ID")

	var results = flag.Bool("printResults", false, "print pipeline configuration of completed pipeline")

	profile = flag.Bool("profile", false, "collect runtime metrics for the pipeline stages")

	var reset = flag.String("reset", "", "reset walrus output back to a known configuration (warning: will roll back repository and delete newer changes)")

	flag.Parse()

	defaultConfigFilename := "pipeline.json"
	if *configFilename == "" {
		log.Println("No pipeline description file set. Using default", defaultConfigFilename)
		*configFilename = defaultConfigFilename
	}
	if *results {
		*printPipeline = true
		*configFilename = *outputDir + "/" + filepath.Base(*configFilename)
	}

	if *diff != "" {
		str, err := lfs.PrintDiff(*outputDir, *diff)
		if err != nil {
			log.Println(err)
		}

		log.Println("Difference from pipeline run " + *diff + ":\n" + str)
		return
	}

	if *reset != "" {
		fmt.Println("Are you sure you want to resset the walrus results?")
		fmt.Println("This will remove all provenance information on files created later (Y/n).")

		var input string
		_, err := fmt.Fscanln(os.Stdin, &input)
		if err != nil {
			log.Println("Could not read input:", err)
			return
		}

		if input == "Y" {
			log.Println("Resetting data to pipeline run", *reset)
			err = lfs.Reset(*outputDir, *reset)
			if err != nil {
				log.Println("Could not reset walrus results to id", *reset, err)
				return
			}
			log.Println("Successfully reset to", *reset)
			log.Println("Any data that was created later than this ID is still available")
			return
		} else {
			return
		}

	}

	if *lfsServer {
		err := lfs.StartServer(*lfsServerDir)
		if err != nil {
			log.Println("Could not start git-lfs server", err)
		} else {
			log.Println("git-lfs server started successfully")
		}
		return
	}

	if *logs != "" {
		stageName := *logs
		filename := *outputDir + "/" + stageName + "/walrus.log"
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Println("Could not read logs for stage: " + stageName)
			log.Println(err)
			return
		}
		log.Println(string(b))
		return
	}

	if *printPipeline {
		p, err := pipeline.ParseConfig(*configFilename)
		switch err.(type) {
		case nil:
			fmt.Println(p)
		case *pipeline.NameError:
			fmt.Println(p)
		default:
			fmt.Println(err)
		}
		return
	}

	// set umask to 000 while walrus is running (we want to have full read/write
	// permissions to the output dirs while running.
	oldmask := syscall.Umask(000)
	defer syscall.Umask(oldmask)

	hostpath, err := filepath.Abs(*outputDir)
	if err != nil {
		log.Println("Check hostpath", err)
		return
	}

	client, err := client.NewEnvClient()
	if err != nil {
		log.Println(err)
		return
	}

	c, err := user.Current()
	if err != nil {
		log.Println(err)
	}
	currentUser = c.Uid + ":" + c.Gid

	p, err := pipeline.ParseConfig(*configFilename)
	if err != nil {
		log.Println(err)
		return
	}

	p.Commit = *commit

	err = fixMountPaths(p.Stages)
	if err != nil {
		log.Println(err)
		return
	}

	err = stopPreviousRun(client, p.Stages)
	if err != nil {
		log.Println(err)
		return
	}

	if *graphFilename != "" {
		err = p.WriteDOT(*graphFilename)
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("DOT graph of the pipeline was written to:", *graphFilename)
		return
	}

	if *web {
		go func() {
			err = startPipelineVisualization(p, *port)
			if err != nil {
				log.Println("Could not start pipeline visualization:", err)
			}
		}()
	}

	err = run(client, p, hostpath, *configFilename)

	if err != nil {
		log.Println(err)
		return
	}

	log.Println("All stages completed successfully. Output written to ",
		hostpath)

	log.Println("Pipeline completed in:", p.Runtime)

	completedPipelineDescription := *outputDir + "/" + filepath.Base(*configFilename)

	err = p.WritePipelineDescription(completedPipelineDescription)
	if err != nil {
		log.Println("ERROR: Could not write pipeline description. ", err)
		return
	}

	if p.Commit {
		err = lfs.Add(*configFilename)
		if err != nil {
			log.Println(err)
			return
		}
		commitId, err := lfs.AddAndCommit(completedPipelineDescription, "Add pipeline configurations")
		if err != nil {
			log.Println(err)
			return
		}
		log.Println("Pipeline completed. Use id", commitId, "to reference it later")
	}
	return
}
