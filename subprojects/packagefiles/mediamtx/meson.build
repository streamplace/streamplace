
project(
  'mediamtx',
  default_options: {},
)
prog_go = find_program('go')

libmediamtx = custom_target(
  'libmediamtx',
  input: ['main-library.go', 'main.go'],
  output: ['mediamtx.a', 'mediamtx.h'],
  command: [
    prog_go,
    'build',
    '-o', '@OUTPUT0@',
    '-buildmode', 'c-archive',
    '-tags', 'netgo',
    '@INPUT@',
  ],
  build_always_stale: true,
  env: {'CGO_ENABLED': '1'},
)
