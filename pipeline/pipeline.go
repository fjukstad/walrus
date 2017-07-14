package pipeline

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

type Pipeline struct {
	Name           string
	Stages         []*Stage
	Comment        string
	Variables      []Variable
	VersionControl bool
}

type Variable struct {
	Name  string
	Value string
}

type Stage struct {
	Name             string
	Image            string
	Entrypoint       []string
	Cmd              []string
	Env              []string
	Inputs           []string
	Volumes          []string
	Parallelism      Parallelism
	Cache            bool
	Comment          string
	MountPropagation string
	Version          string
}

type Parallelism struct {
	Strategy string
	Constant int
}

func ParseConfig(filename string) (*Pipeline, error) {

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	p, err := ReadPipelineDescription(file, filename)
	if err != nil {
		return nil, err
	}

	file, err = FindAndReplaceVariables(p.Variables, file)
	if err != nil {
		return nil, err
	}

	p, err = ReadPipelineDescription(file, filename)
	if err != nil {
		return nil, err
	}

	err = CheckNames(p)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func ReadPipelineDescription(file []byte, filename string) (Pipeline, error) {
	p := Pipeline{}
	var err error

	extension := filepath.Ext(filename)

	if extension == ".json" {
		err = json.Unmarshal(file, &p)
	} else if extension == ".yaml" {
		err = yaml.Unmarshal(file, &p)
	} else {
		return p, errors.New("Pipeline description must be in json or yaml format!")
	}

	return p, err
}

// Finds and replaces all variable names with their respective values. On
// success it returns the file contents of the pipeline description file.
func FindAndReplaceVariables(variables []Variable, file []byte) ([]byte, error) {
	definition := string(file)

	for _, variable := range variables {
		definition = strings.Replace(definition, "{{"+variable.Name+"}}",
			variable.Value, -1)
	}

	return []byte(definition), nil
}

func CheckNames(p Pipeline) error {
	if badName(p.Name) {
		return errors.New("Pipeline name should be a single word without any special characters")
	}

	for _, stage := range p.Stages {
		if badName(stage.Name) {
			return errors.New("Stage name should be a single word without any special characters")
		}
	}
	return nil
}

func badName(name string) bool {
	r, _ := regexp.Compile(`[[:punct:]]`)
	if r.MatchString(name) {
		return true
	}

	r, _ = regexp.Compile(`[[:blank:]]`)
	if r.MatchString(name) {
		return true
	}
	return false
}
