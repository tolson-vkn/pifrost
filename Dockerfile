FROM golang:1.17 AS builder

# For local dev with version working:
#   docker build -t pifrost --build-arg version=$(git describe --abbrev=0) --build-arg gitcommit=$(git rev-parse HEAD) .
ARG gitcommit=UnknownSHA
ARG version=UnknownVER
ARG TARGETARCH

WORKDIR /opt/build
ADD . ./

RUN go test

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -a -tags netgo \
    -ldflags="-X github.com/tolson-vkn/pifrost/version.GitCommit=${gitcommit} -X github.com/tolson-vkn/pifrost/version.Version=${version}"

# ---

FROM alpine

COPY --from=builder /opt/build/pifrost /usr/local/bin/pifrost
ENTRYPOINT ["pifrost"]
