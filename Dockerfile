FROM golang:1.11-alpine as builder

RUN apk update && apk add git make
WORKDIR /go/src/github.com/bitnami-labs/sealed-secrets

RUN go get github.com/bitnami/kubecfg
RUN go get github.com/onsi/ginkgo/ginkgo
COPY . .
RUN make

FROM alpine

COPY --from=builder /go/src/github.com/bitnami-labs/sealed-secrets/controller /usr/local/bin/

ENTRYPOINT ["controller"]
