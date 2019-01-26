# Contributing

## Installation from source

If you just want the latest client tool, it can be installed into
`$GOPATH/bin` with:

```sh
$ go get github.com/bitnami-labs/sealed-secrets/cmd/kubeseal
```

## Developing

To be able to develop on this project, you need to have the following tools installed:
* make
* [Ginkgo](https://onsi.github.io/ginkgo/)
* [Minikube](https://github.com/kubernetes/minikube)
* [kubecfg](https://github.com/ksonnet/kubecfg)
* [goreleaser](https://github.com/goreleaser/goreleaser)
* Go

First fork the repo.  
Then clone the repo using `go get` to preserve paths and add your fork as a remote:
```sh
$ go get github.com/bitnami-labs/sealed-secrets
$ cd $GOPATH/src/github.com/bitnami-labs/sealed-secrets # GOPATH is $HOME/go by default.

$ git remote rename origin upstream
$ git remote add origin <FORK_URL>
```

To build the `kubeseal` and `controller` binaries:
```sh
$ make
```

To build everything; binaries, archives, docker images with `goreleaser`:
```sh
$ make snapshot
```

To run the unit tests:
```bash
$ make test
```

To run the integration tests:
* Start Minikube
* Build the controller for Linux, so that it can be run within a Docker image - edit the Makefile to add 
`GOOS=linux GOARCH=amd64` to `%-static`, and then run `make controller.yaml`
* Alter `controller.yaml` so that `imagePullPolicy: Never`, to ensure that the image you've just built will be
used by Kubernetes
* Add the sealed-secret CRD and controller to Kubernetes - `kubectl apply -f controller.yaml`
* Revert any changes made to the Makefile to build the Linux controller
* Remove the binaries which were possibly built for another OS - `make clean`
* Rebuild the binaries for your OS - `make`
* Run the integration tests - `make integrationtest`

