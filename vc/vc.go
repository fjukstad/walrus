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
			// if no repository found create one in the current working
			// directory.
			if path == "/" {
				path = wd
				fmt.Println("Output directory is not in a git repository. Creating one in " + path)
				repo, err = git.InitRepository(wd, false)
				if err != nil {
					return "", errors.Wrap(err, "Could not initialize git repository")
				}
				break
			}
		} else {
			break
		}
	}

	repositoryLocation := path
	os.Chdir(repositoryLocation)

	dataPath, err := filepath.Rel(repositoryLocation, inputPath)
	if err != nil {
		return "", err
	}

	// ensure git-lfs tracks all files recursively by adding ** pattern, see
	// git PATTERN FORMAT description for more details.
	dataPattern := "" + dataPath + "/**"

	output, err := lfs.Track(dataPattern, repositoryLocation)
	if err != nil {
		return "", errors.Wrap(err, "Could not track files using git-lfs: "+output)
	}

	gitAttr := ".gitattributes"
	changed, err := fileChanged(repo, gitAttr)
	if err != nil {
		return "", err
	}

	if changed {
		_, err := addAndCommit(repo, gitAttr, "update .gitignore to track output files")
		if err != nil {
			return "", err
		}
	}

	index, err := repo.Index()
	if err != nil {
		return "", err
	}

	// traverse the directory and add changed files to the index
	err = filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			changed, err := fileChanged(repo, path)
			if err != nil {
				return err
			}

			if changed {
				err = index.AddByPath(path)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})

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

	currentBranch, err := repo.Head()
	if err != nil {
		return "", err
	}

	currentTip, err := repo.LookupCommit(currentBranch.Target())
	if err != nil {
		return "", err
	}

	var sig = &git.Signature{
		"walrus",
		"walrus@walr.us",
		time.Now(),
	}

	commitId, err := repo.CreateCommit("HEAD", sig, sig, "add data output folder "+dataPath, tree, currentTip)

	return commitId.String(), err
}

func addAndCommit(repo *git.Repository, path, msg string) (string, error) {

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

	var commitId *git.Oid

	var sig = &git.Signature{
		"walrus",
		"walrus@walr.us",
		time.Now(),
	}

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
