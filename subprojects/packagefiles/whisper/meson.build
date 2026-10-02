project('whisper', 'c', 'cpp', version: '1.8.2', default_options: ['c_std=c11', 'cpp_std=c++17', 'warning_level=0'])
cc = meson.get_compiler('c')
cpp = meson.get_compiler('cpp')
inc = include_directories('include', 'src', 'ggml/include', 'ggml/src', 'ggml/src/ggml-cpu')
args = ['-DGGML_USE_CPU', '-DGGML_SCHED_MAX_COPIES=4', '-DWHISPER_VERSION="1.8.2"', '-D_XOPEN_SOURCE=600']
args += ['-DGGML_VERSION="0.9.4"', '-DGGML_COMMIT="4979e04f5dcaccb36057e059bbaed8a2f5288315"']
if host_machine.system() == 'linux'
  args += '-D_GNU_SOURCE'
elif host_machine.system() == 'darwin'
  args += '-D_DARWIN_C_SOURCE'
elif host_machine.system() == 'windows'
  args += ['-D_WIN32_WINNT=0x0602', '-D_CRT_SECURE_NO_WARNINGS']
  args += ['-include', meson.current_source_dir() / 'windows-compat.h']
endif
arch_args = []
arch = 'arm'
if host_machine.cpu_family() == 'x86_64'
  arch = 'x86'
  arch_args = ['-mavx2', '-mfma', '-mf16c']
endif
sources = files(
  'ggml/src/ggml.c', 'ggml/src/ggml.cpp', 'ggml/src/ggml-alloc.c',
  'ggml/src/ggml-backend.cpp', 'ggml/src/ggml-backend-reg.cpp',
  'ggml/src/ggml-opt.cpp', 'ggml/src/ggml-threading.cpp',
  'ggml/src/ggml-quants.c', 'ggml/src/gguf.cpp',
  'ggml/src/ggml-cpu/ggml-cpu.c', 'ggml/src/ggml-cpu/ggml-cpu.cpp',
  'ggml/src/ggml-cpu/repack.cpp', 'ggml/src/ggml-cpu/hbm.cpp',
  'ggml/src/ggml-cpu/quants.c', 'ggml/src/ggml-cpu/traits.cpp',
  'ggml/src/ggml-cpu/amx/amx.cpp', 'ggml/src/ggml-cpu/amx/mmq.cpp',
  'ggml/src/ggml-cpu/binary-ops.cpp', 'ggml/src/ggml-cpu/unary-ops.cpp',
  'ggml/src/ggml-cpu/vec.cpp', 'ggml/src/ggml-cpu/ops.cpp',
  'ggml/src/ggml-cpu/arch/' + arch + '/quants.c',
  'ggml/src/ggml-cpu/arch/' + arch + '/repack.cpp', 'src/whisper.cpp')
deps = [dependency('threads'), cc.find_library('m', required: false)]
if host_machine.system() == 'linux'
  deps += cc.find_library('dl')
endif
# A single archive works for both the shared dev dependency and static releases.
lib = static_library('whisper', sources, include_directories: inc,
  c_args: args + arch_args, cpp_args: args + arch_args,
  dependencies: deps, pic: true, install: true)
# Static release binaries carry the C++ runtime (link-test-linux; Windows
# already links it statically); macOS uses the system libc++.
if host_machine.system() == 'darwin'
  stdlib = cpp.find_library('c++')
elif host_machine.system() == 'windows'
  # Explicit ordering also works with cgo's C linker: libstdc++ introduces
  # winpthread references, and both must be resolved before dynamic linking.
  stdlib = declare_dependency(link_args: [
    '-static-libgcc', '-Wl,-Bstatic', '-lstdc++', '-lwinpthread', '-Wl,-Bdynamic',
  ])
elif get_option('default_library') == 'static'
  # -static-libstdc++ only reaches g++, which meson links release binaries
  # with; cgo links `go test` binaries with gcc, which needs the explicit
  # static library.
  stdlib = declare_dependency(link_args: [
    '-static-libstdc++', '-Wl,-Bstatic', '-lstdc++', '-Wl,-Bdynamic',
  ])
else
  stdlib = cpp.find_library('stdc++')
endif
whisper_dep = declare_dependency(link_with: lib, include_directories: inc, dependencies: deps + [stdlib])
install_headers('include/whisper.h', 'ggml/include/ggml.h', 'ggml/include/ggml-cpu.h',
  'ggml/include/ggml-backend.h', 'ggml/include/ggml-alloc.h', subdir: 'whisper')
import('pkgconfig').generate(lib, name: 'whisper', description: 'CPU whisper.cpp and ggml',
  subdirs: 'whisper', libraries: deps + [stdlib])
