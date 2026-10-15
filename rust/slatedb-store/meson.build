prog_cargo = find_program('cargo')

if get_option('default_library') == 'shared'
  cargo_cmd = '@0@ build'.format(prog_cargo.full_path())

  if host_machine.system() == 'darwin'
    ext = 'dylib'
  elif host_machine.system() == 'linux'
    ext = 'so'
  else
    error('Unsupported dev system: @0@'.format(host_machine.system()))
  endif

  slatedb_store_dep = custom_target(
    'libslatedb_store',
    input: [],
    output: ['libslatedb_store.' + ext],
    command: [
      'sh',
      '-c',
      'cd "@2@" && @0@ && cp "target/debug/libslatedb_store.@3@" "@1@/libslatedb_store.@3@"'.format(cargo_cmd, meson.current_build_dir(), meson.current_source_dir(), ext),
    ],
    build_by_default: true,
    install: true,
    install_dir: get_option('libdir'),
  )
else
  cargo_cmd = '@0@ build --release --target-dir "@1@"'.format(prog_cargo.full_path(), meson.current_build_dir())
  archive_dir = meson.current_build_dir()

  triple = meson.get_external_property('RUST_TRIPLE', '')
  if triple != ''
    cargo_cmd += ' --target @0@'.format(triple)
    archive_dir += '/' + triple
  endif

  slatedb_store_dep = custom_target(
    'libslatedb_store',
    input: [],
    output: ['libslatedb_store.a'],
    command: [
      'sh',
      '-c',
      'cd "@2@" && @0@ && cp "@3@/release/libslatedb_store.a" "@1@/libslatedb_store.a"'.format(cargo_cmd, meson.current_build_dir(), meson.current_source_dir(), archive_dir),
    ],
    build_by_default: true,
    install: true,
    install_dir: get_option('libdir'),
  )
endif