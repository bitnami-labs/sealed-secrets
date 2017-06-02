FROM golang:alpine AS build
COPY . /src
RUN cd /src && CGO_ENABLED=0 go build -o controller ./cmd/controller

FROM alpine
MAINTAINER Angus Lees <gus@inodes.org>
COPY --from=build /src/controller /usr/local/bin/

CMD ["controller", "--logtostderr"]
