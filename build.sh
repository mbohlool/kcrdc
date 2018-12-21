# should be run at the root of the project

VERSION=v15
TARGET="$(cd "$(dirname "${BASH_SOURCE}")/" && pwd -P)"
ARCH=amd64
GOOS=linux
GOLANG_VERSION=latest

docker run --rm -it -v ${TARGET}:/go/src/github.com/mbohlool/kcrdc:Z \
    golang:${GOLANG_VERSION} \
    /bin/bash -c "\
            cd /go/src/github.com/mbohlool/kcrdc && \
            CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(ARCH) go build -a -tags netgo -ldflags '-w -extldflags \"-static\"' -o /go/src/github.com/mbohlool/kcrdc/kcrdc_webhook ."

docker build --pull -t gcr.io/mehdy-k8s/kcrdc-amd64:$VERSION .
docker push gcr.io/mehdy-k8s/kcrdc-amd64:$VERSION
rm kcrdc_webhook