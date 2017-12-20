FROM envoyproxy/envoy:latest

# Augments envoy docker image with the controller binary
ADD controller-linux /controller
ADD envoy/bootstrap.json /
ADD envoy/envoy.jsonnet /

ENTRYPOINT ["/controller"]
