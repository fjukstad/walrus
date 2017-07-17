package lfs

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
)

// git lfs track
// Since the git-lfs devs discourage using git-lfs in go projects we're just
// calling the git-lfs CLI.
func Track(filename, repositoryLocation string) (string, error) {
	cmd := exec.Command("git-lfs", "track", filename)
	cmd.Dir = repositoryLocation
	out, err := cmd.Output()

	// wait to ensure .gitattributes file is up to date.
	// a monument to all my sins.
	time.Sleep(2 * time.Second)

	output := string(out)
	output = strings.TrimRight(output, "\n")

	return output, err
}

// Starts a git-lfs server in a Docker container
func StartServer(mountDir string) error {

	c, err := client.NewEnvClient()
	if err != nil {
		return errors.Wrap(err, "Could not create Docker client")
	}

	image := "fjukstad/lfs-server"
	_, err = c.ImagePull(context.Background(), image,
		types.ImagePullOptions{})

	if err != nil {
		return errors.Wrap(err, "Could not pull iamge")
	}

	hostPath, err := filepath.Abs(mountDir)
	if err != nil {
		return errors.Wrap(err,
			"Could not create absolute git-lfs directory path")
	}

	bind := hostPath + ":/lfs"

	ps := make(nat.PortSet)
	ps["9999/tcp"] = struct{}{}

	pm := make(nat.PortMap)
	pm["9999/tcp"] = []nat.PortBinding{nat.PortBinding{"0.0.0.0", "9999"}}

	resp, err := c.ContainerCreate(context.Background(),
		&container.Config{Image: image,
			ExposedPorts: ps},
		&container.HostConfig{
			Binds:        []string{bind},
			PortBindings: pm},
		&network.NetworkingConfig{},
		"git-lfs-server")

	if err != nil || resp.ID == " " {
		return errors.Wrap(err, "Could not create git-lfs server container")
	}

	containerId := resp.ID

	err = c.ContainerStart(context.Background(), containerId,
		types.ContainerStartOptions{})
	return err

}
