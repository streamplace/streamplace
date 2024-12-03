
project(
  'mediamtx',
  default_options: {},
)
prog_go = find_program('go')

libmediamtx = custom_target(
  'libmediamtx',
  input: ['main.go'],
  output: ['mediamtx.a'],
  command: [
    'bash',
    '-x',
    '-c',
    'export OUT="$(realpath @OUTPUT0@)"'
    + ' && export DIR="$(dirname "$(realpath "@INPUT0@")")"'
    + ' && cp build-flags.go "$DIR"'
    + ' && cd "$DIR"'
    + ' && cp ../../cmd/libaquareum/aquareum.go .'
    + ' && echo "replace aquareum.tv/aquareum => ../.." >> go.mod'
    + ' && cat ../../go.mod | grep replace >> go.mod'
    + ' && go mod tidy'
    + ' && go generate ./...'
    + ' && go build -o $OUT -buildmode c-archive -tags netgo main.go aquareum.go build-flags.go',
  ],
  env: {'CGO_ENABLED': '1'},
  build_always_stale: true,
  build_by_default: true,
)

mediamtx_dep = declare_dependency(
  include_directories: include_directories('.'),
  link_with: [libmediamtx],
)
