from golang:1.10

RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y unzip cmake git


# libgit2
RUN wget https://github.com/libgit2/libgit2/archive/v0.26.0.zip 
RUN unzip v0.26.0.zip 
RUN rm v0.26.0.zip 
WORKDIR libgit2-0.26.0
RUN mkdir build
WORKDIR build
RUN cmake ..
RUN cmake --build . --target install
ENV LD_LIBRARY_PATH=/usr/local/lib

RUN go get gopkg.in/libgit2/git2go.v26

# git lfs
RUN wget https://github.com/git-lfs/git-lfs/releases/download/v2.4.0/git-lfs-linux-amd64-2.4.0.tar.gz
RUN tar -xzf git-lfs-linux-amd64-2.4.0.tar.gz
RUN rm git-lfs-linux-amd64-2.4.0.tar.gz
RUN mv git-lfs-2.4.0/git-lfs /bin
RUN git lfs install

# docker go packages
RUN go get -u github.com/docker/docker/client github.com/docker/distribution
RUN rm -rf $GOPATH/src/github.com/docker/docker/vendor $GOPATH/src/github.com/docker/distribution/vendor

# walrus
RUN mkdir -p $GOPATH/src/github.com/fjukstad/walrus
ADD . $GOPATH/src/github.com/fjukstad/walrus
WORKDIR $GOPATH/src/github.com/fjukstad/walrus
RUN go get ./...

WORKDIR /

ENV DOCKER_API_VERSION=1.35

ENTRYPOINT ["walrus"]
