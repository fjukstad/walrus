package vc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/fjukstad/walrus/lfs"
	git "github.com/libgit2/git2go"
)

// Adds and commits data found in inputPath
func AddAndCommitData(inputPath string) (string, error) {

	var repo *git.Repository
	var err error

	path := inputPath

	defaultRepositoryLocation := popLastDirectory(inputPath)

	// Get current working dir. We'll set it later to the repository location
	// and reset it back to its previous when the function returns.
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	defer os.Chdir(wd)

	// Traverses a directory tree outwards until it a) finds a git repository or
	// b) hits the root (/)
	for {
		repo, err = git.OpenRepository(path)
		if err != nil {
			path = popLastDirectory(path)
			if path == "/" {
				fmt.Println("Output directory is not in a git repository. Creating one.")
				repo, err = git.InitRepository(defaultRepositoryLocation, false)
				if err != nil {
					return "", errors.Wrap(err, "Could not initialize git repository")
				}
				break
			}
		} else {
			fmt.Println("Repository found in ", repo.Path())
			break
		}
	}

	repositoryLocation := path

	os.Chdir(repositoryLocation)

	dataPath, err := filepath.Rel(repositoryLocation, inputPath)
	if err != nil {
		return "", err
	}

	_, err = lfs.Track(dataPath, repositoryLocation)
	if err != nil {
		return "", errors.Wrap(err, "Could not track files using git-lfs")
	}

	gitAttr := ".gitattributes"
	changed, err := fileChanged(repo, gitAttr)
	if err != nil {
		return "", err
	}

	if changed {
		commitId, err := addAndCommit(repo, gitAttr)
		if err != nil {
			return "", err
		}
		return commitId, nil
	}
	return "commitID", nil
}

func addAndCommit(repo *git.Repository, path string) (string, error) {

	index, err := repo.Index()
	if err != nil {
		return "", err
	}

	err = index.AddByPath(path)
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

	walrusSignature := &git.Signature{"walrus",
		"walrus@walr.us",
		time.Now(),
	}

	var commitId *git.Oid

	currentBranch, err := repo.Head()
	if err != nil {
		commitId, err = repo.CreateCommit("HEAD", walrusSignature, walrusSignature, "walrus did this", tree)
	} else {
		currentTip, err := repo.LookupCommit(currentBranch.Target())
		if err != nil {
			return "", err
		}
		commitId, err = repo.CreateCommit("HEAD", walrusSignature, walrusSignature, "walrus did this", tree, currentTip)
	}

	if err != nil {
		return "", err
	}

	err = index.Write()
	if err != nil {
		return "", err
	}

	return commitId.String(), err
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

// Removes the last directory in a path and returns it
func popLastDirectory(path string) string {

	// split the path into a list of dirs /a/b/c --> [a,b,c] then remove
	// the last one and create a new path --> /a/b
	list := strings.Split(path, "/")
	list = list[0 : len(list)-1]
	path = "/" + filepath.Join(list...)
	return path
}
func trackData(path string) error {
	return nil
}

func addData(path string) error {

	return nil
}

func commitData(path string) error {

	return nil
}

func addAndCommitData(path string) error {

	return nil
}
