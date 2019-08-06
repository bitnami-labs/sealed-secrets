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

Unit and e2e-ish testing exists at tests/, needs installed `jsonnet`
and `kubecfg` binaries, as well as a working kubernetes configured
environment for `kubecfg validate` against kubernetes API endpoint.

Above has some basic Travis CI integration (minikube API still WIP),
that exercises unit and golden tests.
