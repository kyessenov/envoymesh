#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -x

echo Building controller
CGO_ENABLED=0 GOOS=linux go build -i -o controller-linux github.com/kyessenov/envoymesh/cmd/controller

echo Building container
docker build -t gcr.io/istio-testing/envoymesh:latest .
