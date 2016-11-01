# Walrus
Walrus is a small tool I built to run data analysis pipelines using Docker
containers. It is very simple: Walrus reads a pipeline description from either a
JSON or YAML file and starts Docker containers as described in this file. It's
really nothing more than a fancy shell script to start Docker containers.




## Pipeline 
A pipeline has a *name*, a list of *pipeline stages*, a *version*, and a
*comment*. See below for an example pipeline. 

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

Walrus also writes the pipeline description to the output `walrus/.walrus`
directory on the host if users want to investigate the pipeline that created the
output. 

## Parallelism
Pipeline stages that could be run in parallel are run in parallel by default. 

## Pipeline Versions
Users can use `Version` field in the pipeline description to keep track of
different versions of a pipeline. Before starting a new pipeline Walrus checks
the output directory for any output from previous runs and makes a hard copy of
any output data it finds.  These are moved to directories named
`STAGENAME-VERSION/` in the output directory using the `Version` field from the
pipeline description. If the version field is not set it generates a version
name for the pipeline. 

# Installation and usage

- Install [go](http://golang.org). 
- `go get github.com/fjukstad/walrus`
- `walrus -f $PIPELINE_DESCRIPTION` where `$PIPELINE_DESCRIPTION` is the
  filename of a pipeline description you've created.  

```
$ walrus --help
Usage of walrus:
  -f string
    	pipeline description file (default "pipeline.json")
  -output string
    	where walrus should store output data on the host (default "walrus")
```

# Example pipeline
Here's a small example pipeline. It consists of two stages: the first writes all
filenames in the `/` directory to a file `/walrus/stage1/file`, the second writes
all files with `bin` in the name to a new file `/walrus/stage2/file2`. 

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
version: 0.1
comment: This is the first example pipeline!
```

# Name
Because every data analysis framework has to be named after a big animal.
Right? 

> There is something remarkably fantastic and prehistoric about these monsters. I could not help thinking of a merman, or something of the kind, as it lay there just under the surface of the water, blowing and snorting for quite a long while at a time, and glaring at us with its round glassy eyes. 
> - Fridtjof Nansen on walruses 
