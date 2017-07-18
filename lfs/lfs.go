package lfs

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	git "github.com/libgit2/git2go"
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

// Adds and commits data found in inputPath
func AddAndCommitData(path, msg string) (string, error) {

	repositoryLocation, err := Add(path)
	if err != nil {
		return "", err
	}

	commitId, err := commit(repositoryLocation, msg)
	if err != nil {
		return "", err
	}

	return commitId, nil
}

// Add the given path to the
func Add(path string) (string, error) {

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer os.Chdir(wd)

	repo, repositoryLocation, err := openRepository(path)
	if err != nil {
		return "", err
	}

	os.Chdir(repositoryLocation)

	dataPath, err := filepath.Rel(repositoryLocation, path)
	if err != nil {
		return "", err
	}

	// ensure git-lfs tracks all files recursively by adding ** pattern, see
	// git PATTERN FORMAT description for more details.
	dataPattern := "" + dataPath + "/**"

	gitAttr := ".gitattributes"

	// if pattern already exists don't rerun the track command
	b, err := ioutil.ReadFile(gitAttr)
	if err != nil {
		pe := err.(*os.PathError)
		if pe.Err.Error() != "no such file or directory" {
			return "", err
		}
	}

	if !strings.Contains(string(b), dataPattern) {
		output, err := Track(dataPattern, repositoryLocation)
		if err != nil {
			return "", errors.Wrap(err, "Could not track files using git-lfs: "+output)
		}
	}

	changed, err := fileChanged(repo, gitAttr)
	if err != nil {
		return "", err
	}

	if changed {
		err := addToIndex(repo, gitAttr)
		if err != nil {
			return "", err
		}
	}

	output, err := add(dataPath, repositoryLocation)
	if err != nil {
		return "", errors.Wrap(err, "Could not add files "+output)
	}

	return repositoryLocation, nil
}

func addToIndex(repo *git.Repository, path string) error {

	index, err := repo.Index()
	if err != nil {
		return err
	}

	err = index.AddByPath(path)
	if err != nil {
		return err
	}

	_, err = index.WriteTree()
	if err != nil {
		return err
	}

	err = index.Write()
	if err != nil {
		return err
	}

	return err
}

// Removes the last directory in a path and returns it
func popLastDirectory(path string) string {

	// split the path into a list of dirs /a/b/c --> [a,b,c] then remove
	// the last one and create a new path --> /a/b
	list := strings.Split(path, "/")
	list = list[0 : len(list)-1]
	path = "/" + filepath.Join(list...)
	return path
}

// Returns true if file is new, modified or deleted
func fileChanged(repo *git.Repository, path string) (bool, error) {
	status, err := repo.StatusFile(path)
	if err != nil {
		return false, err
	}

	if status == git.StatusWtNew || status == git.StatusWtModified ||
		status == git.StatusWtDeleted {
		return true, nil
	}

	return false, nil
}

// commits staged changes
func commit(path, msg string) (string, error) {

	repo, err := git.OpenRepository(path)
	if err != nil {
		return "", err
	}

	index, err := repo.Index()
	if err != nil {
		return "", err
	}

	treeId, err := index.WriteTree()
	if err != nil {
		return "", err
	}

	tree, err := repo.LookupTree(treeId)
	if err != nil {
		return "", err
	}

	err = index.Write()
	if err != nil {
		return "", err
	}

	var sig = &git.Signature{
		"walrus",
		"walrus@github.com/fjukstad/walrus",
		time.Now(),
	}

	var commitId *git.Oid

	currentBranch, err := repo.Head()
	if err != nil {
		commitId, err = repo.CreateCommit("HEAD", sig, sig, msg, tree)
	} else {
		currentTip, err := repo.LookupCommit(currentBranch.Target())
		if err != nil {
			return "", err
		}
		commitId, err = repo.CreateCommit("HEAD", sig, sig, msg, tree, currentTip)
	}
	if err != nil {
		return "", err
	}

	err = index.Write()
	if err != nil {
		return "", err
	}

	return commitId.String(), nil
}

// Will try to open a git repository located at the given path. If it is not
// found it will traverse the directory tree outwards until i either finds a
// repository or hits the root. If no repository is found it will initialize one
// in the current working directory.
func openRepository(path string) (repo *git.Repository, repositoryPath string, err error) {

	wd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	for {
		repo, err = git.OpenRepository(path)
		if err != nil {
			path = popLastDirectory(path)
			// Root hit
			if path == "/" {
				path = wd
				fmt.Println("Output directory is not in a git repository. Creating one in " + path)
				repo, err = git.InitRepository(wd, false)
				if err != nil {
					return nil, "", errors.Wrap(err, "Could not initialize git repository")
				}
				break
			}
		} else {
			break
		}
	}
	return repo, path, nil
}

// git add
// To speed up dev time for the prototype, use the exec pkg not git2go package
// to add files. Future versions will get rid of this hacky way of doing things
// by creating the blobs, softlinks etc. but that's for later!
func add(path, repositoryLocation string) (string, error) {
	cmd := exec.Command("git", "add", path)
	cmd.Dir = repositoryLocation
	out, err := cmd.Output()
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
