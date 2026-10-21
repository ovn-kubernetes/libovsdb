# Match ovn-kubernetes' Go builder and Fedora runtime image conventions.
ARG BUILDER_IMAGE=quay.io/projectquay/golang:1.26@sha256:d5acf07940fb5e6e2d07542abd853cece623d95ffd364769ec825aab5471d3b8
ARG RUNTIME_IMAGE=quay.io/fedora/fedora:44@sha256:a32cc6149ce88510a5e23ce249d42681d61758d5851c398d29a76e5c94d1ed72

FROM --platform=$BUILDPLATFORM ${BUILDER_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -o /out/ ./cmd/...

FROM ${RUNTIME_IMAGE} AS runtime
WORKDIR /work

FROM runtime AS stress
COPY --from=build /out/stress /usr/local/bin/stress
CMD ["stress"]

FROM runtime AS print_schema
COPY --from=build /out/print_schema /usr/local/bin/print_schema
CMD ["print_schema"]

FROM runtime AS modelgen
COPY --from=build /out/modelgen /usr/local/bin/modelgen
CMD ["modelgen"]
