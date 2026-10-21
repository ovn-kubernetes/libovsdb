# Container images

The root `Containerfile` builds the three command-line tools. It follows
ovn-kubernetes's image conventions: `quay.io/projectquay/golang:1.26` for the
builder and `quay.io/fedora/fedora:44` for the runtime. Runtime images contain
only the selected tool, not the Go toolchain or repository sources.

| Target | Published image | Default command |
| --- | --- | --- |
| `stress` | `ghcr.io/ovn-kubernetes/libovsdb/stress` | `stress` |
| `modelgen` | `ghcr.io/ovn-kubernetes/libovsdb/modelgen` | `modelgen` |
| `print_schema` | `ghcr.io/ovn-kubernetes/libovsdb/print-schema` | `print_schema` |

Build and run locally (Docker or Podman):

```sh
docker build -f Containerfile --target modelgen -t libovsdb-modelgen .
docker run --rm libovsdb-modelgen modelgen --help
```

The Go binaries are statically compiled for the requested target platform.
Buildx can build both architectures without emulating the Go compiler:

```sh
docker buildx build -f Containerfile --target stress \
    --platform linux/amd64,linux/arm64 .
```

Base images are pinned to multi-architecture manifest digests for reproducible
builds. Update those digests when updating base images. Override `BUILDER_IMAGE`
and `RUNTIME_IMAGE` with build arguments if needed.
Keep the Go builder version compatible with `go.mod` and the runtime version
aligned with ovn-kubernetes.

## CI and publishing

`.github/workflows/tool-images.yml` is shared by the CI and release workflows.
It builds all three images for `linux/amd64` and `linux/arm64`:

- Pull requests: build only, without logging into or publishing to a registry.
- Merges to upstream `main`: publish `latest` after the CI test jobs pass.
- Release tags: publish the exact tag (for example, `v0.9.1`) before creating the
  GitHub release. Release builds do **not** move `latest`, including prereleases.

Publishing is restricted to `ovn-kubernetes/libovsdb` on `main` or tag refs.
GHCR authentication uses `GITHUB_TOKEN` with `packages: write`; no additional
registry secret is required. Repository/package administrators must allow this
repository to write the packages and make the packages public if anonymous
pulls are desired. No registry settings are changed by these workflows.

## OVS image

`ovs/Containerfile` uses Fedora 44 for both the build and runtime stages. The
existing OVS image workflow continues to publish
`ghcr.io/ovn-kubernetes/ovs` using OVS version tags and its existing `latest`
policy; libovsdb release tags apply only to the three tool images.

```sh
docker build -f ovs/Containerfile --build-arg OVS_VERSION=v3.7.1 \
    -t libovsdb-ovs ovs
```
