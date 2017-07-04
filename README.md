# Walrus
Walrus is a small tool I built to run data analysis pipelines using Docker
containers. It is very simple: Walrus reads a pipeline description from either a
JSON or YAML file and starts Docker containers as described in this file. 

## Pipeline 
A pipeline has a *name*, a list of *pipeline stages*, and optional
*comments*. See below for an example pipeline. 

## Pipeline stage
A pipeline stage has a *name*, a *Docker image* it is based on, a list of
pipeline stages that it depends on (e.g. if it relies on output from these), and
a *command* it runs on start up. 

## IO
Each pipeline stage should write any output data to the directory
`/walrus/STAGENAME` that is automatically mounted onside the docker container
on start-up. Walrus automatically mounts input directories from its dependencies
on start-up at `/walrus/INPUT_STAGENAME`. The user specifies where this
`/walrus` directory is on the host OS by using the `-output` command line flag
(see Usage for more information).
On default it writes everything to a `walrus` directory in the current working
directory of where the user executes the walrus command. 

## Parallelism
Pipeline stages that could be run in parallel are run in parallel by default. 

# Installation and usage

- Install [go](http://golang.org). 
- `go get github.com/fjukstad/walrus`
- `walrus -f $PIPELINE_DESCRIPTION` where `$PIPELINE_DESCRIPTION` is the
  filename of a pipeline description you've created.  

```
$ walrus --help 
Usage of walrus:
  -i string
    	pipeline description file (default "pipeline.json")
  -o string
    	where walrus should store output data on the host (default "walrus")
  -p string
    	port to run web server for pipeline visualization (default ":9090")
  -web
    	host interactive visualization of the pipeline
```

# Example pipeline
Here's a small example pipeline. It consists of two stages: the first writes all
filenames in the `/` directory to a file `/walrus/stage1/file`, the second writes
all filenames with `bin` in the name to a new file `/walrus/stage2/file2`. 

```
name: example
stages:
- name: stage1
  image: ubuntu:latest
  cmd:
  - sh
  - -c
  - ls / > /walrus/stage1/file
- name: stage2
  image: ubuntu:14.04
  cmd:
  - sh
  - -c
  - grep bin /walrus/stage1/file > /walrus/stage2/file2
  inputs:
  - stage1
comment: This is the first example pipeline!
```

# Name
Because every data analysis framework has to be named after a big animal.
Right? 

> There is something remarkably fantastic and prehistoric about these monsters. I could not help thinking of a merman, or something of the kind, as it lay there just under the surface of the water, blowing and snorting for quite a long while at a time, and glaring at us with its round glassy eyes. 
> - Fridtjof Nansen on walruses 
