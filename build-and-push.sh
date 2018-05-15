#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

DOCKER_HUB="${DOCKER_HUB:-gcr.io/istio-testing}"
DOCKER_TAG="${DOCKER_TAG:-latest}"

echo Building sidecar
CGO_ENABLED=0 GOOS=linux go build -o docker/agent-linux github.com/kyessenov/envoymesh/cmd/agent
cp bootstrap.jsonnet docker/
docker build -f docker/Dockerfile.sidecar -t ${DOCKER_HUB}/envoysidecar:${DOCKER_TAG} docker

echo Building controller
CGO_ENABLED=0 GOOS=linux go build -o docker/controller-linux github.com/kyessenov/envoymesh/cmd/controller
docker build -f docker/Dockerfile.mesh -t ${DOCKER_HUB}/envoymesh:${DOCKER_TAG} docker

echo Pushing images
docker push ${DOCKER_HUB}/envoysidecar:${DOCKER_TAG}
docker push ${DOCKER_HUB}/envoymesh:${DOCKER_TAG}
