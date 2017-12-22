#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -x

pushd docker

echo Building sidecar
docker build -f Dockerfile.sidecar -t gcr.io/istio-testing/envoysidecar:latest .
docker push gcr.io/istio-testing/envoysidecar:latest

echo Building controller
CGO_ENABLED=0 GOOS=linux go build -i -o controller-linux github.com/kyessenov/envoymesh/cmd/controller
cp ../envoy.jsonnet .
docker build -f Dockerfile.mesh -t gcr.io/istio-testing/envoymesh:latest .
docker push gcr.io/istio-testing/envoymesh:latest

popd
