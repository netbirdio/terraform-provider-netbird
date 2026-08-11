#!/usr/bin/env bash
# Builds the NetBird images the acceptance suite runs against from the netbird
# version pinned in go.mod, so the server the provider is tested against is the
# same revision its client library was generated from. A published image can
# drift from that revision; a build from the pinned module cannot.
#
# The module cache holds the component Dockerfiles, so the build context is the
# extracted module directory. The client has no source Dockerfile upstream (the
# published one packages a goreleaser artifact), so this repo carries one.
#
# Tags are printed to stdout as "component=image" lines and build output goes to
# stderr, so a caller can do:
#
#   docker save $(test/build-images.sh | cut -d= -f2)
#
# Environment:
#   NB_E2E_SERVER_IMAGE      use this image instead of building. A value with a
#   NB_E2E_PROXY_IMAGE       "/" is a registry reference, echoed unchanged for
#   NB_E2E_CLIENT_IMAGE      the caller to pull; a bare tag is built under that
#   NB_E2E_DASHBOARD_IMAGE   name. The dashboard is never built (it lives in
#                            another repository) and defaults to the published
#                            image.
#   NB_E2E_REBUILD_IMAGES=1  rebuild even when the tag already exists locally.
set -euo pipefail

readonly module="github.com/netbirdio/netbird"
readonly default_dashboard_image="netbirdio/dashboard:latest"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

version="$(go list -m -f '{{.Version}}' "$module")"
src="$(go list -m -f '{{.Dir}}' "$module")"
if [ -z "$src" ] || [ ! -d "$src" ]; then
  echo "downloading $module@$version into the module cache" >&2
  go mod download "$module"
  src="$(go list -m -f '{{.Dir}}' "$module")"
fi
if [ -z "$src" ] || [ ! -d "$src" ]; then
  echo "cannot locate the source of $module@$version" >&2
  exit 1
fi

# pull_image fetches a registry reference so the tag exists locally, which is
# what lets a caller `docker save` every image this script reports. An image that
# is already present is left alone: it may be a tag built here and never pushed.
pull_image() {
  local component="$1" image="$2"

  if docker image inspect "$image" >/dev/null 2>&1; then
    echo "$component: reusing $image" >&2
  else
    echo "$component: pulling $image" >&2
    docker pull --quiet "$image" >&2
  fi
  echo "$component=$image"
}

# resolve_image echoes the image to use for a component and makes sure it is
# present locally, building it when it is ours to build. $1 component,
# $2 override env value, $3 default local tag, $4 dockerfile (relative to the
# build context, or absolute).
resolve_image() {
  local component="$1" override="$2" tag="$3" dockerfile="$4"

  if [ -n "$override" ]; then
    if [[ "$override" == */* ]]; then
      pull_image "$component" "$override"
      return
    fi
    tag="$override"
  fi

  if [ "${NB_E2E_REBUILD_IMAGES:-}" != "1" ] && docker image inspect "$tag" >/dev/null 2>&1; then
    echo "$component: reusing $tag" >&2
  else
    echo "$component: building $tag from $module@$version" >&2
    DOCKER_BUILDKIT=1 docker build -f "$dockerfile" -t "$tag" "$src" >&2
  fi
  echo "$component=$tag"
}

resolve_image server "${NB_E2E_SERVER_IMAGE:-}" "netbird-server:$version" \
  "$src/combined/Dockerfile.multistage"
resolve_image proxy "${NB_E2E_PROXY_IMAGE:-}" "netbird-reverse-proxy:$version" \
  "$src/proxy/Dockerfile.multistage"
resolve_image client "${NB_E2E_CLIENT_IMAGE:-}" "netbird-client:$version" \
  "$repo_root/test/Dockerfile.client"

pull_image dashboard "${NB_E2E_DASHBOARD_IMAGE:-$default_dashboard_image}"
