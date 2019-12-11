################################################################################
# BUILDER/DEVELOPMENT IMAGE
################################################################################
FROM golang:1.12-alpine as builder

# Add git for downloading dependencies
RUN apk add --no-cache git gcc g++ libc-dev

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

COPY main.go ./
COPY script/* ./script/

RUN go build

################################################################################
# LINT IMAGE
################################################################################

FROM golang:1.12 as ci

# Ensure we run all go commands against the vendor folder
ENV GOFLAGS -tags=ci

# Install linter
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.15.0

WORKDIR /build

COPY --from=builder /build .
COPY .golangci.yml .

RUN go mod download

################################################################################
# FINAL IMAGE
################################################################################

FROM alpine:3.10

ENV BUILD_DIR=/build

WORKDIR /app

RUN apk add --no-cache ca-certificates bash

COPY --from=builder $BUILD_DIR/ovh-ip-updater-go ${BUILD_DIR}/script/run.sh ./

RUN chmod +x run.sh

CMD ["./run.sh"]
