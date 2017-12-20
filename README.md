# envoymesh

Envoy mesh is an experimental simple service mesh built on top of
[go-control-plane](https://github.com/envoyproxy/go-control-plane) that
provides the following features:

- lightweight installation, targeted for Kubernetes
- transparent proxy injection
- out-of-the-box telemetry, authorization checks, and L7 routing capabilities
- direct access to [Envoy xDS
  APIs](https://github.com/envoyproxy/data-plane-api) for customizing
  application-level network behavior

## Build instructions

envoymesh uses standard go tooling. Requirements:
- golang 1.9.2 or above
- godep

Use `build.sh` script to generate a sidecar container that includes
[envoy](https://www.envoyproxy.io/) and a controller binary.

## Test instructions

TODO

