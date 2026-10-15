#!/usr/bin/env bash
set -euo pipefail

# Build independent locked graphs: Iroh's RustCrypto prereleases cannot share
# Cargo resolution with MoQ's stable RustCrypto dependencies. Compose the rlibs
# with rustc, not two staticlibs, so there is exactly one Rust standard library.
source_dir=$(cd "$1" && pwd -P)
mkdir -p "$2"
output_dir=$(cd "$2" && pwd -P)
profile=${3:-release}
triple=${4:-}
crate_type=${5:-staticlib}

# Resolve the rustup proxy once, before either Cargo invocation. Both graphs and
# the final staticlib must use the same compiler, including its standard library.
rustc_command=${RUSTC:-rustc}
export RUSTC="$("$rustc_command" --print sysroot)/bin/rustc"
if [[ -z "$triple" ]]; then
  while IFS=' ' read -r key value; do
    if [[ "$key" == host: ]]; then triple=$value; break; fi
  done < <("$RUSTC" -vV)
fi

if [[ "$profile" != debug && "$profile" != release ]]; then
  echo "unsupported native profile: $profile" >&2
  exit 1
fi
if [[ ! -f "$source_dir/subprojects/moq/meson.build" ]]; then
  echo "MoQ source is missing; run make native-source first" >&2
  exit 1
fi

# A final non-Cargo link cannot consume linker-plugin LTO artifacts. Keep the
# exact same flags for both graphs and the final composition. Abort is also the
# upstream FFI panic policy: a Rust panic must not unwind through Go.
if [[ -n "${CARGO_ENCODED_RUSTFLAGS:-}" ]]; then
  IFS=$'\x1f' read -r -a rustflags <<< "$CARGO_ENCODED_RUSTFLAGS"
else
  read -r -a rustflags <<< "${RUSTFLAGS:-}"
fi
rustflags+=(-C lto=off -C embed-bitcode=no -C panic=abort)
printf -v CARGO_ENCODED_RUSTFLAGS '%s\x1f' "${rustflags[@]}"
export CARGO_ENCODED_RUSTFLAGS=${CARGO_ENCODED_RUSTFLAGS%$'\x1f'}
export CARGO_PROFILE_DEV_LTO=false CARGO_PROFILE_RELEASE_LTO=false

cargo_args=(build --locked --target "$triple")
if [[ "$profile" == release ]]; then cargo_args+=(--release); fi
cargo "${cargo_args[@]}" --manifest-path "$source_dir/Cargo.toml" \
  --package iroh-streamplace --target-dir "$output_dir/iroh"
cargo "${cargo_args[@]}" --manifest-path "$source_dir/subprojects/moq/Cargo.toml" \
  --package moq-ffi --target-dir "$output_dir/moq"

case "$triple" in
  *-apple-darwin) dylib_ext=dylib ;;
  *-linux-*) dylib_ext=so ;;
  *) dylib_ext= ;;
esac
compose_args=()
case "$crate_type" in
  staticlib) ext=a ;;
  cdylib)
    ext=$dylib_ext
    # One library under both names: link-time lookups of either resolve to
    # this file, recorded once by its own name.
    if [[ "$ext" == so ]]; then
      compose_args+=(-C link-arg=-Wl,-soname,libstreamplace_native.so)
    else
      compose_args+=(-C "link-arg=-Wl,-install_name,$output_dir/libstreamplace_native.dylib")
    fi
    ;;
  *)
    echo "unsupported native crate type: $crate_type" >&2
    exit 1
    ;;
esac
out="$output_dir/libstreamplace_native.$ext"
iroh_rlib="$output_dir/iroh/$triple/$profile/libiroh_streamplace.rlib"
moq_rlib="$output_dir/moq/$triple/$profile/libmoq_ffi.rlib"

# Composing is a full link of both graphs, so it only runs when Cargo rebuilt
# an rlib or the composition itself changed.
fresh=true
for input in "$iroh_rlib" "$moq_rlib" "$source_dir/rust/streamplace-native/src/lib.rs" "${BASH_SOURCE[0]}"; do
  [[ "$out" -nt "$input" ]] || fresh=false
done
if [[ "$fresh" != true ]]; then
  # Native C/C++ static dependencies are bundled in the rlibs; target system
  # libs are propagated by Meson's streamplacedeps.pc. A staticlib also prints
  # rustc's complete list, so a dependency change can be checked against that
  # explicit system-lib list. With --target, proc-macro dylibs live in the
  # separate host profile/deps directories. rustc needs those too when loading
  # either rlib's crate metadata.
  "$RUSTC" --edition=2021 --crate-name streamplace_native --crate-type "$crate_type" \
    --target "$triple" "${rustflags[@]}" ${compose_args[@]+"${compose_args[@]}"} --print native-static-libs \
    -L "dependency=$output_dir/iroh/$triple/$profile/deps" \
    -L "dependency=$output_dir/moq/$triple/$profile/deps" \
    -L "dependency=$output_dir/iroh/$profile/deps" \
    -L "dependency=$output_dir/moq/$profile/deps" \
    --extern "iroh_streamplace=$iroh_rlib" \
    --extern "moq_ffi=$moq_rlib" \
    "$source_dir/rust/streamplace-native/src/lib.rs" \
    -o "$out.tmp"
  mv "$out.tmp" "$out"
  # Both names resolve to the same objects, not two runtimes.
  ln -f "$out" "$output_dir/libmoq_ffi.$ext.tmp"
  mv "$output_dir/libmoq_ffi.$ext.tmp" "$output_dir/libmoq_ffi.$ext"
fi
# The other kind, from a build of the other type, must not shadow this one.
for stale in a "$dylib_ext"; do
  if [[ -n "$stale" && "$stale" != "$ext" ]]; then
    rm -f "$output_dir/libstreamplace_native.$stale" "$output_dir/libmoq_ffi.$stale"
  fi
done
# The old generator only reads Iroh's UniFFI 0.28 metadata, never MoQ's 0.32.
if [[ -n "$dylib_ext" ]]; then
  ln -f "$output_dir/iroh/$triple/$profile/libiroh_streamplace.$dylib_ext" \
    "$output_dir/libiroh_streamplace.$dylib_ext"
fi
