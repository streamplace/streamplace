#!/usr/bin/env bash
set -euo pipefail

# Build the locked graphs independently: Iroh's RustCrypto prereleases cannot
# share Cargo resolution with MoQ's stable dependencies. Compose rlibs with
# rustc, not separate staticlibs, so there is exactly one Rust standard library.
source_dir=$(cd "$1" && pwd -P)
mkdir -p "$2"
output_dir=$(cd "$2" && pwd -P)
profile=${3:-release}
triple=${4:-}
crate_type=${5:-staticlib}
station_release=${6:-false}

# Resolve the rustup proxy once. All graphs and the final library must use the
# same compiler, including its standard library.
rustc_command=${RUSTC:-rustc}
export RUSTC="$("$rustc_command" --print sysroot)/bin/rustc"
rust_host=
if [[ -z "$triple" || "$station_release" == true ]]; then
  while IFS=' ' read -r key value; do
    if [[ "$key" == host: ]]; then rust_host=$value; break; fi
  done < <("$RUSTC" -vV)
fi
triple=${triple:-$rust_host}

if [[ "$profile" != debug && "$profile" != release ]]; then
  echo "unsupported native profile: $profile" >&2
  exit 1
fi
case "$station_release" in
  false) ;;
  true)
    if [[ "$rust_host" != *-linux-* || "$triple" != "$rust_host" ||
          "$crate_type" != staticlib || "$profile" != release ]]; then
      echo "station_release requires a native Linux static release build" >&2
      exit 1
    fi
    ;;
  *)
    echo "unsupported station_release value: $station_release" >&2
    exit 1
    ;;
esac
if [[ ! -f "$source_dir/subprojects/moq/meson.build" ]]; then
  echo "MoQ source is missing; run make native-source first" >&2
  exit 1
fi

# A final non-Cargo link cannot consume linker-plugin LTO artifacts. Use the
# same target flags for all graphs and the final composition. A Rust panic must
# not unwind through Go.
if [[ -n "${CARGO_ENCODED_RUSTFLAGS:-}" ]]; then
  IFS=$'\x1f' read -r -a rustflags <<< "$CARGO_ENCODED_RUSTFLAGS"
else
  read -r -a rustflags <<< "${RUSTFLAGS:-}"
fi
rustflags+=(-C lto=off -C embed-bitcode=no -C panic=abort)
printf -v CARGO_ENCODED_RUSTFLAGS '%s\x1f' "${rustflags[@]}"
export CARGO_ENCODED_RUSTFLAGS=${CARGO_ENCODED_RUSTFLAGS%$'\x1f'}
export CARGO_PROFILE_DEV_LTO=false CARGO_PROFILE_RELEASE_LTO=false

cargo_args=(--locked --target "$triple")
if [[ "$profile" == release ]]; then cargo_args+=(--release); fi
cargo build "${cargo_args[@]}" --manifest-path "$source_dir/Cargo.toml" \
  --package iroh-streamplace --target-dir "$output_dir/iroh"
cargo build "${cargo_args[@]}" --manifest-path "$source_dir/subprojects/moq/Cargo.toml" \
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
alias_out="$output_dir/libmoq_ffi.$ext"
station_mode_file="$output_dir/streamplace-native.$ext.station-release"
iroh_rlib="$output_dir/iroh/$triple/$profile/libiroh_streamplace.rlib"
moq_rlib="$output_dir/moq/$triple/$profile/libmoq_ffi.rlib"
inputs=("$iroh_rlib" "$moq_rlib" "$source_dir/rust/streamplace-native/src/lib.rs" "${BASH_SOURCE[0]}")
if [[ "$station_release" == true ]]; then
  station_dir="$output_dir/station-slate"
  station_rlib="$station_dir/$triple/$profile/libstation_slate.rlib"
  # Cargo's internal metadata salt namespaces host proc macros and target
  # crates. With --target, RUSTFLAGS alone cannot cover the host units.
  __CARGO_DEFAULT_LIB_METADATA=streamplace-station-slate \
    cargo rustc "${cargo_args[@]}" --manifest-path "$source_dir/rust/station-slate/Cargo.toml" \
      --lib --crate-type rlib --target-dir "$station_dir"
  compose_args+=(
    --cfg station_release
    -L "dependency=$station_dir/$triple/$profile/deps"
    -L "dependency=$station_dir/$profile/deps"
    --extern "station_slate=$station_rlib"
  )
  inputs+=("$station_rlib")
fi

# Reuse the link only when its inputs and station mode still match.
fresh=true
[[ "$out" -ef "$alias_out" && -f "$station_mode_file" &&
  "$(<"$station_mode_file")" == "$station_release" ]] || fresh=false
for input in "${inputs[@]}"; do
  [[ "$out" -nt "$input" ]] || fresh=false
done
if [[ "$fresh" != true ]]; then
  # Native C/C++ static dependencies are bundled in the rlibs; target system
  # libs are propagated by Meson's streamplacedeps.pc. A staticlib also prints
  # rustc's complete list, so a dependency change can be checked against that
  # explicit system-lib list. With --target, proc-macro dylibs live in the
  # separate host profile/deps directories. Iroh and MoQ keep distinct locked
  # rustversion versions to avoid a pre-existing StableCrateId collision.
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
  # Invalidate the mode before replacing either artifact.
  rm -f "$station_mode_file"
  mv "$out.tmp" "$out"
  # Both names resolve to the same objects, not two runtimes.
  ln -f "$out" "$alias_out.tmp"
  mv "$alias_out.tmp" "$alias_out"
  printf '%s\n' "$station_release" > "$station_mode_file"
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
