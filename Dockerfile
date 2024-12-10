# Build the manager binary
FROM --platform=linux/amd64 public.int.repositories.cloud.sap/golang:1.23.2-alpine as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Set the GOPROXY to the internal SAP Artifactory. Needed for xMAke
ARG GOPROXY="https://int.repositories.cloud.sap/artifactory/api/go/goproxy-virtual"
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY pkg/controller/ pkg/controller/
COPY charts charts
# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o squid-installer main.go

# The Deployment which we use overwrites the entrypoint without specifying full path.
# In thistroless there is no PATH variable set.
# We have to remove the entrypoint from the Deployment and use the entrypoint.

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
# FROM gcr.io/distroless/static:nonroot
# WORKDIR /
# COPY --from=builder /workspace/squid-installer .
# # Usless
# COPY --from=builder /workspace/squid-installer /bin/squid-installer
# ENV PATH="${PATH}:/bin:/bin/squid-installer"

# USER 65532:65532

# ENTRYPOINT ["/squid-installer"]

# TODO: Remove the entrypoint from the Deployment and use the distroless build
FROM public.int.repositories.cloud.sap/com.sap.edgelm/security-patched-alpine:0.21.0@sha256:ee77123e2f1fc75365c38edf5ca0fe4928d2944f7a1ddd7752e23e3d6a7f0aec

ENV OPERATOR=/usr/local/bin/squid-installer \
    USER_UID=65532 \
    USER_NAME=squid-installer

# install operator binary
COPY --from=builder /workspace/squid-installer .
COPY --from=builder /workspace/squid-installer /usr/local/bin

RUN adduser -D -u ${USER_UID} ${USER_NAME}

USER ${USER_UID}

ENTRYPOINT ["/squid-installer"]
