FROM envoyproxy/envoy:latest

# Augments envoy docker image with the controller binary
ADD controller-linux /controller
ADD bootstrap.json /
ADD envoy.jsonnet /

ENTRYPOINT ["/controller"]
