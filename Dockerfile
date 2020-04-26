FROM golang:1.13.7

ADD go.mod /go/src/github.com/minio/bottlenet/go.mod
ADD go.sum /go/src/github.com/minio/bottlenet/go.sum
WORKDIR /go/src/github.com/minio/bottlenet/
# Get dependencies - will also be cached if we won't change mod/sum
RUN go mod download

ADD . /go/src/github.com/minio/bottlenet/
WORKDIR /go/src/github.com/minio/bottlenet/

ENV CGO_ENABLED=0

RUN go build -ldflags '-w -s' -a -o bottlenet .

FROM scratch
MAINTAINER MinIO Development "dev@min.io"
EXPOSE 7761

COPY --from=0 /go/src/github.com/minio/bottlenet/bottlenet /bottlenet

ENTRYPOINT ["/bottlenet"]
