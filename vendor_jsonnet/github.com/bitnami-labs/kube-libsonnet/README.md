[![Build Status](https://travis-ci.org/bitnami-labs/kube-libsonnet.svg?branch=master)](https://travis-ci.org/bitnami-labs/kube-libsonnet)
# kube-libsonnet

This repo has been originally populated by the `lib/` folder contents
from `https://github.com/bitnami-labs/kube-manifests` as of Mar/2018,
aiming to provide a library of `jsonnet` manifests for common
Kubernetes objects (such as `Deployment`, `Service`, `Ingress`, etc).

Accordingly, above `kube-manifests` has been changed to use this repo as
a git submodule, i.e.:

    $ git submodule add https://github.com/bitnami-labs/kube-libsonnet
    $ cat .gitmodules
    [submodule "lib"]
    path = lib
    url = https://github.com/bitnami-labs/kube-libsonnet

## Testing

Unit and e2e-ish testing at tests/, needs usable `docker-compose`
setup at node, will run a `k3s` "dummy" container to serve Kube API,
"enough" to run `kubecfg validate` against it:

    make tests

If you don't want that full kube-api stack (will then use your "local"
kubernetes configured environment), you can run:

    make -C tests local-tests kube-validate
