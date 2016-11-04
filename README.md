# Kompose (Kubernetes + Compose)

[![Build Status](https://travis-ci.org/kubernetes-incubator/kompose.svg?branch=master)](https://travis-ci.org/kubernetes-incubator/kompose)
[![Join us in #kompose on k8s Slack](https://s3.eu-central-1.amazonaws.com/ngtuna/join-us-on-slack.png)](http://slack.kubernetes.io) in #kompose channel

`kompose` is a tool to help users who are familiar with `docker-compose` move to [Kubernetes](http://kubernetes.io). `kompose` takes a Docker Compose file and translates it into Kubernetes resources.

`kompose` is a convenience tool to go from local Docker development to managing your application with Kubernetes. Transformation of the Docker Compose format to Kubernetes resources manifest may not be exact, but it helps tremendously when first deploying an application on Kubernetes.


## OpenCompose

If you have come to this branch of kompose that means, you want to try out OpenCompose. Thank you. :)

This is an initial PoC for OpenCompose implementation for Kubernetes and OpenShift. We plan to merge this code with mainline kompose project in near future. Since Kompose architechture is such that it can understand multiple inputs, it was easy to add support for OpenCompose.

The user experience remains the same and you can follow the rest of the readme on how to use this version of `kompose` tool. All you need to do is use the correct input type.

`kompose --opencompose services.yml` should do the trick. To learn more and see a complete walkthough you can go [here](http://developers.redhat.com/blog/2016/11/03/container-orchestration-specification-for-better-devops/) or  [![asciicast](https://asciinema.org/a/7f7dw37n37m5kfn7v9uh1pn1w.png)](https://asciinema.org/a/7f7dw37n37m5kfn7v9uh1pn1w)



## Use Case

If you have a Docker Compose [`docker-compose.yml`](./examples/docker-compose.yml) or a Docker Distributed Application Bundle [`docker-compose-bundle.dab`](./examples/docker-compose-bundle.dab) file, you can convert it into Kubernetes deployments and services like this:

```console
$ kompose --bundle docker-compose-bundle.dab convert
WARN[0000]: Unsupported key networks - ignoring
file "redis-svc.json" created
file "web-svc.json" created
file "web-deployment.json" created
file "redis-deployment.json" created

$ kompose -f docker-compose.yml convert
WARN[0000]: Unsupported key networks - ignoring
file "redis-svc.json" created
file "web-svc.json" created
file "web-deployment.json" created
file "redis-deployment.json" created
```

Other examples are provided in the _examples_ [directory](./examples).

## Installation

Grab the latest [release](https://github.com/kubernetes-incubator/kompose/releases) for your OS, untar and extract the binary.

Linux example:
```
wget https://github.com/kubernetes-incubator/kompose/releases/download/v0.1.1/kompose_linux-amd64.tar.gz
tar -xvf kompose_linux-amd64.tar.gz --strip 1
sudo mv kompose /usr/local/bin
```

## Bash completion
Running this below command in order to benefit from bash completion

```console
$ PROG=kompose source script/bash_autocomplete
```

## Building

### Building with `go`

- You need `make`
- You need `go` v1.6 or later.
- If your working copy is not in your `GOPATH`, you need to set it accordingly.

You can either build via the Makefile:

```console
$ make binary
```

Or `go build`:

```console
$ go build -tags experimental -o kompose ./cli/main
```

You need `-tags experimental` because the current `bundlefile` package of docker/libcompose is still experimental.

If you have `go` v1.5, it's still good to build `kompose` with the following settings:

```console
$ CGO_ENABLED=0 GO15VENDOREXPERIMENT=1 go build -o kompose -tags experimental ./cli/main
```

To create a multi-platform binary, use the `binary-cross` command via `make`:

```console
$ make binary-cross
```

## Contributing and Issues

`kompose` is a work in progress, we will see how far it takes us. We welcome any pull request to make it even better.
If you find any issues, please [file it](https://github.com/kubernetes-incubator/kompose/issues).

## Community, discussion, contribution, and support

We follow the Kubernetes community principles.

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack](http://slack.kubernetes.io): #kompose

Incubation of `kompose` into the Kubernetes project is shepherd in the [SIG-APPS](https://github.com/kubernetes/community).

[@ericchiang](https://github.com/ericchiang) is acting as Champion for [incubation](https://github.com/kubernetes/community/blob/master/incubator.md)

We do a bi-weekly community meeting. Here is the link to [agenda doc](https://docs.google.com/document/d/1I5I21Cp_JZ9Az5MgMcu6Hl7m8WQ1Eqk_WeQLHenNom0/edit?usp=sharing).

Meeting link: [https://bluejeans.com/404059616](https://bluejeans.com/404059616)

## RoadMap

* September 15th 2016: Propose to Kubernetes incubator.
* September 30th 2016: Make the first official release of `kompose`, 0.1.0
* October 1st 2016: Add _build_ support connected to a private registry run by Kubernetes
* October 15th 2016: Add multi-container Pods, PVC and service types support.
* November 1st 2016: Add preference file to specify preferred resources for conversion and preferred provider.
* November 15th 2016: Improve support for Docker bundles to target specific image layers.
* December 24th 2016: Second release of `kompose`, 0.2.0

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
