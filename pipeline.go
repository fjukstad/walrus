package main

type Pipeline struct {
	Name   string
	Stages []Stage
	Cache  bool
}

type Stage struct {
	Image       string
	Cmd         []string
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
