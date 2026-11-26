#!/usr/bin/env bash
#
# Seed a fresh checkout from the warm dev image's baked build.
#
#   hack/warm-seed.sh        # inside a container, cwd = checkout
#
# Containers made from the warm image (hack/warm-image.sh) mount the checkout
# at /streamplace and carry a full build tree at /opt/streamplace-warm, built
# from that same absolute path. Copying it in lets `make provision` skip the
# cold build. No-op on builder-image containers and on checkouts that already
# have a build.
set -euo pipefail

cd "$(dirname "$0")/.."
# shellcheck source=hack/lib/frontend-inputs.sh
. hack/lib/frontend-inputs.sh
WARM=/opt/streamplace-warm

if [[ ! -d "$WARM" ]]; then
  exit 0
fi

if [[ -d build-linux-amd64 ]]; then
  echo "warm seed: build-linux-amd64 already present; nothing to seed"
  exit 0
fi

COMMIT="$(cat "$WARM/commit" 2>/dev/null || true)"
if [[ -z "$COMMIT" ]] || ! git cat-file -e "${COMMIT}^{commit}" 2>/dev/null; then
  echo "warm seed: image commit '${COMMIT}' is not in this checkout; skipping" >&2
  echo "warm seed: run 'make provision' again after fetching, or cold-build here" >&2
  exit 0
fi

# The image's files can carry foreign uids from archive extraction; the
# checkout must stay owned by the user running the build.
COPY=(cp -a --no-preserve=ownership)

echo "warm seed: copying build tree from $WARM ($(git rev-parse --short "$COMMIT"))"
"${COPY[@]}" "$WARM/build-linux-amd64" .

# Only downloaded subproject trees: wraps and patches are tracked source, and
# an existing tree (or a local-dev symlink) must win over the image's.
mkdir -p subprojects .build
for entry in "$WARM"/subprojects/*; do
  name="$(basename "$entry")"
  if [[ -d "$entry" && ! -L "$entry" && ! -e "subprojects/$name" && ! -L "subprojects/$name" ]]; then
    "${COPY[@]}" "$entry" subprojects/
  fi
done
if [[ -d "$WARM/.build/bin" ]]; then
  mkdir -p .build/bin
  "${COPY[@]}" "$WARM/.build/bin/." .build/bin/
fi
if [[ -f "$WARM/.build/lexicon-stamp" ]]; then
  cp -a --no-preserve=ownership "$WARM/.build/lexicon-stamp" .build/
fi
for d in js/app/dist js/web/dist; do
  if [[ -d "$WARM/$d" ]]; then
    rm -rf "$d"
    "${COPY[@]}" "$WARM/$d" "$d"
  fi
done

# Ninja and make use mtimes to decide what is stale. The seeded outputs are as
# old as the image build, which can be older than this checkout's files, so
# mark them current first; then touch only what differs from the image's commit
# so those rebuild. The sleep keeps the two batches distinguishable at
# filesystem-timestamp resolution.
touch .build/lexicon-stamp
for d in js/app/dist js/web/dist; do
  if [[ -f "$d/index.html" ]]; then
    touch "$d/index.html"
  fi
done
sleep 1
git diff --name-only -z "$COMMIT" | while IFS= read -r -d '' f; do
  if [[ -e "$f" ]]; then
    touch "$f"
  fi
done

# A deleted or added frontend input has nothing for the mtime check above to
# see, so the seeded bundle would keep the image's content. Force a rebuild
# when the frontend inputs differ from the commit the bundle was built from.
frontend_diff="$(git diff --name-only -z "$COMMIT" -- "${FRONTEND_INPUTS[@]}" | tr -d '\0')"
frontend_untracked="$(git ls-files --others --exclude-standard -z -- "${FRONTEND_INPUTS[@]}" | tr -d '\0')"
if [[ -n "$frontend_diff" || -n "$frontend_untracked" ]]; then
  rm -f js/app/dist/index.html js/web/dist/index.html
  echo "warm seed: frontend inputs differ from the image; bundles will rebuild"
fi

echo "warm seed: done"
