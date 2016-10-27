package main

import (
	"encoding/json"
	"io/ioutil"
)

type Pipeline struct {
	Name   string
	Stages []Stage
	Cache  bool
}

type Stage struct {
	Name        string
	Image       string
	Cmd         []string
	Env         []string
	Stdin       []string
	Inputs      []Input
	Parallelism Parallelism
}

type Parallelism struct {
	Strategy string
	Constant int
}

type Input struct {
	Name string
}

func ParseConfig(filename string) (*Pipeline, error) {
	p := Pipeline{}
	file, e := ioutil.ReadFile(filename)
	if e != nil {
		return nil, e
	}
	err := json.Unmarshal(file, &p)
	return &p, err
}
