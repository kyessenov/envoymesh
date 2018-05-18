# Pre-release 0.8 istio sidecar
# FROM gcr.io/istio-testing/proxy_debug:9417254427d882a9e394e8509b895d952afe4aab
FROM ubuntu:xenial
ADD envoy /usr/local/bin/envoy
ADD agent-linux /agent
ADD bootstrap.jsonnet /
ENTRYPOINT ["/agent"]
