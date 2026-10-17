# Download/patch source at configure time; later offline Cargo builds use the
# pinned checkout and its upstream Cargo.lock. This subproject compiles nothing.
subproject('moq')

rust_triple = meson.get_external_property('RUST_TRIPLE', '')
native_profile = 'release'
# Development builds link a shared library: the Go binary loads it at run
# time, so a Go-only change relinks against a small stub, not the whole debug
# Rust graph. Shipped and cross builds stay static.
native_crate_type = 'staticlib'
native_libs_ext = 'a'
if get_option('default_library') == 'shared'
  native_profile = 'debug'
  if host_machine.system() == 'linux'
    native_crate_type = 'cdylib'
    native_libs_ext = 'so'
  elif host_machine.system() == 'darwin'
    native_crate_type = 'cdylib'
    native_libs_ext = 'dylib'
  endif
endif

# Cargo linker/cc-crate settings accept one executable, not an argv array.
# Preserve launcher arguments and Meson's target/sysroot options in wrappers.
native_cc_command = meson.get_compiler('c').cmd_array()
native_cpp_command = meson.get_compiler('cpp').cmd_array()
native_commands = {
  'cc': native_cc_command + get_option('c_args'),
  'cxx': native_cpp_command + get_option('cpp_args'),
  'linker': native_cc_command + get_option('c_args') + get_option('c_link_args'),
}
foreach kind, command : native_commands
  quoted_command = []
  foreach arg : command
    quoted_command += '\'' + arg.replace('\'', '\'"\'"\'') + '\''
  endforeach
  wrapper = configure_file(
    input: 'compiler-wrapper.sh.in',
    output: 'native-' + kind,
    configuration: {'COMMAND': ' '.join(quoted_command)},
  )
  run_command(find_program('chmod'), '+x', wrapper, check: true)
endforeach
native_cc = meson.current_build_dir() + '/native-cc'
native_cpp = meson.current_build_dir() + '/native-cxx'
native_linker = meson.current_build_dir() + '/native-linker'
prog_native_go = find_program('../../hack/native-go.sh')

native_env = environment()
native_env.set('CC', native_cc)
native_env.set('CXX', native_cpp)
native_env.set('AR', find_program('ar', native: false).full_path())
if rust_triple != ''
  native_env.set('CARGO_TARGET_' + rust_triple.to_upper().replace('-', '_') + '_LINKER', native_linker)
endif
if host_machine.system() == 'darwin'
  # moq-ffi's default codecs require ScreenCaptureKit (macOS 12.3).
  native_env.set('MACOSX_DEPLOYMENT_TARGET', '12.3')
endif

native_libdir = meson.current_build_dir()
native_libs = custom_target(
  'streamplace-native',
  output: ['libstreamplace_native.' + native_libs_ext, 'libmoq_ffi.' + native_libs_ext],
  command: [
    find_program('bash'),
    files('../../hack/build-native.sh'),
    meson.project_source_root(),
    native_libdir,
    native_profile,
    rust_triple,
    native_crate_type,
  ],
  env: native_env,
  build_always_stale: true, # Cargo, not Ninja, tracks the two Rust source graphs.
  build_by_default: true,
  install: true,
  install_dir: get_option('libdir'),
)
streamplace_native_dep = native_libs[0]
