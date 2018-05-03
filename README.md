![walrus](misc/walrus-200x200.png)

# walrus
walrus is a small tool for executing data analysis pipelines using Docker
containers. It is very simple: walrus reads a pipeline description from either a
JSON or YAML file and starts Docker containers as described in this file. We
have used walrus to develop analysis pipelines for analyzing whome-exome as well
as RNA sequencing datasets. 

# Pipeline 
A pipeline has a *name*, a list of *pipeline stages*, optional
*comments* and *variables*. See below for an example pipeline. 

## Pipeline stage
A pipeline stage has a *name*, a *Docker image* it is based on, a list of
pipeline stages that it depends on (e.g. if it relies on output from these), and
a *command* it runs on start up. 

## IO
Each pipeline stage should write any output data to the directory
`/walrus/STAGENAME` that is automatically mounted onside the docker container
on start-up. walrus automatically mounts input directories from its dependencies
on start-up at `/walrus/INPUT_STAGENAME`. The user specifies where this
`/walrus` directory is on the host OS by using the `-output` command line flag
(see Usage for more information).
On default it writes everything to a `walrus` directory in the current working
directory of where the user executes the walrus command. 

## Parallelism
Pipeline stages that could be run in parallel are run in parallel by default. 

## Variables
You can declare variables in the pipeline description as well. You declare these
as `{"Name": "variableName", "Value": "variableValue"}` and use them in the
pipeline description by wrapping them like this `{{variableName}}`. See
[pipeline.json](https://github.com/fjukstad/walrus/blob/master/example/fruit_stand_variables/pipeline.json)
for an example. 

# Reproducible pipelines 
## Tools
Since walrus requires that tools are packaged within Docker containers, it
provides a simple mechanism to ensure a reproducible execution envirionment. 

## Parameters
We reccommend that you use [git](https://git-scm.com/) to version control your
pipeline descriptions. This will ensure that you can keep track of the different 
parameters to the different tools as you develop your analysis pipeline.

## Data
walrus automatically tracks data in the pipeline with
[git-lfs](https://git-lfs.github.com/). When users start a pipeline walrus will
track any output data from any of the pipeline stages and commit them to the
repository versioning the pipeline description, If you do not have a repository
walrus will set one up for you.

Using git to version control your pipeline data is completely
optional, and  users can of course opt out of versioning data with `git-lfs` by
using the `walrus -version-control=false` parameter. 

git-lfs requires a server for hosting the large files, and while
[Github](https://help.github.com/articles/about-git-large-file-storage/),
[BitBucket](https://confluence.atlassian.com/bitbucket/git-large-file-storage-in-bitbucket-829078514.html)
provide hosting opportunities, we have added a `-lfs-server` flag that starts a
local [git-lfs-server](https://github.com/fjukstad/lfs-server) for use with
`git-lfs`. Users can use this server to store files with `git-lfs` or push them
to some other remote. 

### Performance
You may experience that `git-lfs` uses some time to start keeping
track of your data. Adding the NA12878 WGS (270GB) bam file takes roughly 1 hour
on our fat server (80 Intel xenon CPUs, 10 cores/CPU, ~1TB memory). Bear in mind
that `git-lfs` runs on a single CPU. Most of the time spent is simply copying
the data into the `.git/lfs` folder. Hopefully this will improve in later
versions of `git-lfs`. 

# Installation and usage
There are two options for installing and using walrus: install walrus and its
dependencies natively on your system, or use our walrus Docker image. It may
sound a bit silly to have a Docker container orchestrate other containers, but
by sharing the Docker socket (`/var/run/docker.sock`) with the walrus container
it works!  There are
[drawbacks](https://www.lvh.io/posts/dont-expose-the-docker-socket-not-even-to-a-container.html)
to sharing the Docker socket and we only encourage this approach if you want to
try out walrus without thinking about setting up your own environment. 


## Native
### Prerequisites and dependencies 
We are working on simplifying the installation process. In short you need to
install [go](http://golang.org), [git-lfs](https://git-lfs.github.com/),
[libgit2](https://github.com/libgit2/libgit2),
[git2go](https://github.com/libgit2/git2go), and the Docker Go packages before
you can install walrus. You also need [cmake](https://cmake.org/) to compile
libgit2 (install it via your preferred package manager. 

#### Go 
Follow the [instructions on golang.org](https://golang.org/doc/install) to
install Go. You also need to set up your
[GOPATH](https://github.com/golang/go/wiki/SettingGOPATH). 

#### Libgit2 and git2go
First install `libgit`, specifically version 26.
```
    wget https://github.com/libgit2/libgit2/archive/v0.26.0.zip
    unzip v0.26.0.zip 
    cd libgit2-0.26.0/

    mkdir build && cd build
    cmake ..
    cmake --build . --target install
```

Make sure that you have added the install directory to your `LD_LIBRARY_PATH`
before continuing. For example, like this: 

```
    echo "export LD_LIBRARY_PATH=/usr/local/lib" >> ~/.bash_profile
```

After `libgit2` is installed you can install version 26 of `git2go`

```
    go get gopkg.in/libgit2/git2go.v26
```

#### git-lfs
Install `git-lfs` following the instructions on the
[git-lfs](https://git-lfs.github.com/) homepage.

### Docker Go packages 
We need to do some wrangling of the Docker Go packages before we can install
walrus.  First download the packages, then remove the `vendor` directories
before continuing. 

```
    go get -u github.com/docker/docker github.com/docker/distribution
    rm -rf $GOPATH/src/github.com/docker/docker/vendor $GOPATH/src/github.com/docker/distribution/vendor
```

### walrus 

```
    go get github.com/fjukstad/walrus
```

### Usage 
Once you have installed walrus you can start analyzing data with 

```
    walrus -i $PIPELINE_DESCRIPTION
```

where `$PIPELINE_DESCRIPTION` is the filename of a
pipeline description you've created. For more details run `$ walrus --help`. 

## Docker 
There's only a single command needed to start analyzing data using the walrus
Docker container. Let's assume you have a `pipeline.json` pipeline description
in your working directory. You can analyze it by running

```
    docker run -v /var/run/docker.sock:/var/run/docker.sock -v $(pwd):$(pwd) -t fjukstad/walrus -i $(pwd)/pipeline.json -o $(pwd)/output
```

and it will write the output to a directory `output/` in your current working
directory. 


While there's a single command you also have to take special care when
specifying the volumes in your pipeline description.  You must use the full
path, not just relative path, where your data is on your host. 

Below is a short example to analyze the [fruit_stand
example](example/fruit_stand), that assumes that you have downloaded walrus to
your `GOPATH`. Before you can run the pipeline you have to modify one line of
the first stage in [pipeline.json](example/fruit_stand/pipeline.json) from

```
    "Volumes": ["data:/data"],
```

to 

```
    "Volumes": ["GOPATH/src/github.com/fjukstad/walrus/example/fruit_stand/data:/data"],
```

where you have to substitute `GOPATH` with your actual GOPATH. If your data is
elsewhere you'll have to substitute the path with the full path on your system.
Once you have updated the path you can then run the pipeline using

```
    docker run -v /var/run/docker.sock:/var/run/docker.sock -v $(pwd):$(pwd) -t fjukstad/walrus -i $(pwd)/pipeline.json -o $(pwd)/output
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
