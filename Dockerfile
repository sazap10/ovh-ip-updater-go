################################################################################
# BUILDER/DEVELOPMENT IMAGE
################################################################################
FROM golang:1.11-alpine as builder

# Add git for downloading dependencies
RUN apk add --no-cache git gcc g++ libc-dev

WORKDIR /build

COPY go.mod go.sum ./

RUN go mod download

ADD main.go ./

RUN go build

################################################################################
# LINT IMAGE
################################################################################

FROM golang:1.11 as ci

# Provide stracktraces from all user go routines
ENV GOTRACEBACK all

# Ensure we run all go commands against the vendor folder
ENV GOFLAGS -mod=vendor

# Set the go path
ENV GOPATH /gopath
ENV PATH="/${GOPATH}/bin:${PATH}"

# Install gometalinter
RUN curl -L https://git.io/vp6lP | sh

WORKDIR /gopath/src/github.com/sazap10/ovh-ip-updater-go

COPY --from=builder /build .
COPY .gometalinter.json .
RUN GO111MODULE=on go mod vendor

# Lint code
RUN gometalinter ./... --vendor

# Run tests
# RUN go test ./... -race -timeout 30m -p 1

################################################################################
# DEBUG IMAGES
################################################################################

FROM alpine:3.8 as debug

RUN apk add --no-cache libc6-compat

COPY --from=builder /build/mongo-sidecar /app/
COPY --from=builder /go/bin/dlv /

CMD [ "/dlv", "exec", "-l", ":2345", "--headless=true", "--log=true", "--api-version=2", "/app/mongo-sidecar" ]

################################################################################
# FINAL IMAGE
################################################################################

FROM alpine:3.8

ENV BUILD_DIR=/build

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder $BUILD_DIR/ovh-ip-updater-go .

CMD ["./ovh-ip-updater-go"]