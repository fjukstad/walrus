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

var parallelIdentifier string = "_parallel_"

// Parses the pipeline configuration and returns the pipeline. It will verify
// that names are valid, find and replace variable names and create parallel
// pipeline stages.
func ParseConfig(filename string) (*Pipeline, error) {

	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	p, err := ReadPipelineDescription(file, filename)
	if err != nil {
		return nil, err
	}

	err = CheckNames(p)
	if err != nil {
		return nil, err
	}

	p, err = FindAndReplaceVariables(p, file)
	if err != nil {
		return nil, err
	}

	p.FixDependencies()

	return &p, nil
}

// Read a pipeline description.
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

// Write a pipeline description to a file.
func (p Pipeline) WritePipelineDescription(filename string) error {
	var b []byte
	var err error

	switch extension := filepath.Ext(filename); extension {
	case ".json":
		b, err = json.Marshal(p)
	case ".yaml":
		b, err = yaml.Marshal(p)
	}

	err = ioutil.WriteFile(filename, b, 06666)
	if err != nil {
		return err
	}
	return nil

}

// Since we support parallelizing stages, a stage has to wait for automatically
// generated stages. E.g. if we have parallelized stage input_A and input_B,
// both with following stages process_A and process_B. process_A will have to
// wait for input_A, but the pipeline definition will only list `input` as a
// dependency. This function fixes and cleans up the "dependency graph".
// NOTE TO SELF: refactor this one jesus christ it's bad.
func (p Pipeline) FixDependencies() {
	for _, stage := range p.Stages {
		// This is a parallelized stage, we'll need to find any dependent stages
		// and update their list of "inputs"
		if strings.Contains(stage.Name, parallelIdentifier) {
			originalName := strings.Split(stage.Name, parallelIdentifier)[0]
			parallelName := strings.Split(stage.Name, parallelIdentifier)[1]
			for _, dependentStage := range p.Stages {
				if dependentStage.Name != stage.Name {
					// Stage has parallel stage as a dependency
					if sliceContains(dependentStage.Inputs, originalName) {
						// Dependent stage is itself a parallel stage. Find and
						// replace all occurences of the 'non-parallel' name
						// with the newly updated one
						if strings.HasSuffix(dependentStage.Name, parallelIdentifier+parallelName) {
							dependentStage.Inputs = sliceReplaceMatching(dependentStage.Inputs, originalName, stage.Name, -1)
						} else if !strings.Contains(dependentStage.Name, parallelIdentifier) {
							// If dependent stage has any parallel stages as
							// input, add the stage to this list.
							if sliceContains(dependentStage.Inputs, parallelIdentifier) {
								dependentStage.Inputs = append(dependentStage.Inputs, stage.Name)
							} else {
								// if not find and replace all matching
								// 'non-parallel' names with new parallelized one
								dependentStage.Inputs = sliceReplace(dependentStage.Inputs, originalName, stage.Name, -1)
							}
						}
					}
				}
			}
		}
	}
}

// Stringify a pipeline description.
func (p Pipeline) String() string {
	str := "Name:" + p.Name
	str += "Stages:\n"
	for _, stage := range p.Stages {
		str += stage.String()
	}
	return str
}

// Stringify a pipeline stage.
func (stage Stage) String() string {
	str := stage.Name + "\n"
	str += "\t Image: " + stage.Image + "\n"
	str += "\t Entrypoint: " + strings.Join(stage.Entrypoint, "") + "\n"
	str += "\t Cmd: " + strings.Join(stage.Cmd, " ") + "\n"
	str += "\t Env: " + strings.Join(stage.Env, " ") + "\n"
	str += "\t Inputs: " + strings.Join(stage.Inputs, " ") + "\n"
	str += "\t Volumes: " + strings.Join(stage.Volumes, " ") + "\n"
	//str +=\t  "Parallelism:" + stage.Parallelism + "\n"
	//str +=\t  "Cache:" + stage.Cache + "\n"
	//str +=\t  "Mount Propagation:" + stage.MountPropagation + "\n"
	str += "\t Comment: " + stage.Comment + "\n"
	str += "\t Version: " + stage.Version
	str += "\n"
	return str
}

// Checks if a string maches an item within a slice.
func inSlice(s []string, substr string) bool {
	for _, str := range s {
		if str == substr {
			return true
		}
	}
	return false
}

// Checks if items in a slice contain a specific string.
func sliceContains(s []string, substr string) bool {
	for _, str := range s {
		if strings.Contains(str, substr) {
			return true
		}
	}
	return false
}

// Replaces occurences of `old` with `new` in the slice `s`. Users can specify
// the number of replacements in each string with the `n` parameter.
func sliceReplace(s []string, old, new string, n int) []string {
	var replaced []string
	for _, str := range s {
		replaced = append(replaced, strings.Replace(str, old, new, n))
	}
	return replaced
}

func sliceReplaceMatching(s []string, old, new string, n int) []string {
	var replaced []string
	for _, str := range s {
		if old == str {
			replaced = append(replaced, strings.Replace(str, old, new, n))
		} else {
			replaced = append(replaced, str)
		}
	}
	return replaced

}

// Finds and replaces all variable names with their respective single values. On
// success it returns the file contents of the pipeline description file. For
// multi-value variables it will create one stage per variable value. We assume
// that these can run concurrently.
func FindAndReplaceVariables(p Pipeline, file []byte) (Pipeline, error) {

	// Find and replace all variables. If a variable has mulitple definitios
	// walrus will create one stage per definition.

	for _, stage := range p.Stages {
		for _, variable := range p.Variables {
			if sliceContains(stage.Cmd, "{{"+variable.Name+"}}") {
				// If the variable has only got one deifnition simply find and
				// replace it. If the variable is a list then we need to make
				// one stage per variable definition.
				if len(variable.Values) > 1 {
					for _, value := range variable.Values {
						var temp_stage Stage = *stage

						temp_stage.Cmd = sliceReplace(temp_stage.Cmd, "{{"+variable.Name+"}}", value, -1)
						temp_stage.Name = stage.Name + "_parallel_" + value
						temp_stage.remove = false

						p.Stages = append(p.Stages, &temp_stage)
						stage.remove = true
					}
				} else {
					value := variable.Values[0]
					stage.Cmd = sliceReplace(stage.Cmd, "{{"+variable.Name+"}}", value, -1)
				}
			}
		}
	}

	var stages []*Stage

	// Remove old stages that have been split into parallel stages.
	for _, stage := range p.Stages {
		if !stage.remove {
			stages = append(stages, stage)
		}
	}

	p.Stages = stages

	return p, nil
}

// Verify that the pipeline name and pipeline stage names are valid.
func CheckNames(p Pipeline) error {
	if badName(p.Name) {
		return errors.New("Pipeline name: '" + p.Name + "' should be a single word without any special characters")
	}

	for _, stage := range p.Stages {
		if badName(stage.Name) {
			return errors.New("Stage name: '" + stage.Name + "' should be a single word without any special characters")
		}

		if strings.Contains(stage.Name, parallelIdentifier) {
			return errors.New("Stage name: '" + stage.Name + "' shuold not contain " + parallelIdentifier)
		}
	}
	return nil
}

func badName(name string) bool {
	r, _ := regexp.Compile(`\W`)
	if r.MatchString(name) {
		return true
	}

	r, _ = regexp.Compile(`[[:blank:]]`)
	if r.MatchString(name) {
		return true
	}

	return false
}
