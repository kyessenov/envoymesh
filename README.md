# envoymesh

Envoy mesh is an experimental simple service mesh built on top of
[go-control-plane](https://github.com/envoyproxy/go-control-plane) that
provides the following features:

- sidecar-based service mesh architecture
- lightweight installation, targeted for Kubernetes
- out-of-the-box telemetry, authorization checks, and L7 routing capabilities
- direct access to [Envoy xDS
  APIs](https://github.com/envoyproxy/data-plane-api) for customizing
  application-level network behavior

## Goals
- Minimal implementation of a control plane for a fleet of Envoy proxies
- ADS for coordinated configuration rollout
- Implementation of native Envoy extension point (access log, metrics, authz)

## Limitations

- This project uses jsonnet extensively for rapid prototyping of Envoy API
  processing logic.
- To simplify the deployment model, sidecar container includes both the proxy
  and its controller.

## Build instructions

envoymesh uses standard go tooling. Requirements:
- golang 1.9.2 or above
- godep

Use `build.sh` script to generate a sidecar container that includes
[envoy](https://www.envoyproxy.io/) and a controller binary.

## Test instructions

Requirements for integration testing:
- helm

TODO

