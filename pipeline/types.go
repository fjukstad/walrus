package pipeline

import "time"

type Pipeline struct {
	Name      string
	Stages    []*Stage
	Comment   string
	Variables []Variable
	Commit    bool
	Runtime   time.Duration
	Version   string
}

type Variable struct {
	Name   string
	Values []string
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
	remove           bool
	Runtime          time.Duration
}

type Parallelism struct {
	Strategy string
	Constant int
}
