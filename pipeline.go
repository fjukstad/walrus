package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"regexp"
)

type Pipeline struct {
	Name    string
	Stages  []Stage
	Cache   bool
	Version string
	Comment string
}

type Stage struct {
	Name        string
	Image       string
	Cmd         []string
	Env         []string
	Stdin       []string
	Inputs      []string
	Parallelism Parallelism
}

type Parallelism struct {
	Strategy string
	Constant int
}

func ParseConfig(filename string) (*Pipeline, error) {
	p := Pipeline{}
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(file, &p)
	if err != nil {
		return nil, err
	}
	err = CheckNames(p)
	if err != nil {
		return nil, err
	}
	return &p, nil
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
