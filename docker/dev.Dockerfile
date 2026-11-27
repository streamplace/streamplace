# syntax=docker/dockerfile:1
#
# Warm dev image: one full `make dev` of a commit, baked into an image that new
# checkouts can seed from instead of cold-building. This is not a shipped
# image; it is built locally by `make warm-image` (hack/warm-image.sh) and
# consumed by hack/warm-seed.sh.
#
# The build tree is made at /streamplace because that is where new containers
# bind-mount the checkout. Meson, Cargo, and pkg-config bake absolute paths into
# their outputs, so the path must match when the tree is copied into a checkout.
ARG BUILDER_IMAGE
FROM ${BUILDER_IMAGE}

ARG WARM_COMMIT=unknown
LABEL org.streamplace.warm.commit=$WARM_COMMIT

WORKDIR /streamplace
COPY . /streamplace

# The frontend build records git metadata, so hack/warm-image.sh supplies a
# real repository with HEAD at WARM_COMMIT; it is not needed in the image.
# node_modules and the pnpm store are dropped too: checkouts install them from
# the host store, and the seed does not copy them.
#
# A container bind-mounts the checkout at /streamplace, hiding this tree, so
# move it beside the mount for hack/warm-seed.sh to copy in. The move must
# happen in this layer: renaming a directory that a lower layer owns would
# copy the whole tree into a second layer.
RUN make dev-setup \
 && make dev \
 && pnpm --filter @streamplace/e2e-web install-browser \
 && find /streamplace -name node_modules -prune -exec rm -rf {} + \
 && store="$(pnpm store path)" \
 && if [ -n "$store" ]; then rm -rf "$store"; fi \
 && rm -rf /streamplace/.git \
 && echo "$WARM_COMMIT" > /opt/warm-commit \
 && mv /streamplace /opt/streamplace-warm \
 && mv /opt/warm-commit /opt/streamplace-warm/commit

WORKDIR /
