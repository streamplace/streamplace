// Both UniFFI versions keep their own ABI and namespace. One rustc invocation
// composes their independently locked rlibs with a single Rust standard library.
// UniFFI 0.28's macro emits a bare no_mangle with the defining crate's 2024
// edition. Spell its identical reexport body with the required unsafe syntax.
#[unsafe(no_mangle)]
pub extern "C" fn iroh_streamplace_uniffi_reexport_hack() {
    iroh_streamplace::uniffi_reexport_hack()
}
moq_ffi::uniffi_reexport_scaffolding!();
