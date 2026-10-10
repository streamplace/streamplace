project(
  'obs-streamplace',
  'c',
  default_options: ['c_std=gnu11', 'warning_level=2', 'werror=true', 'buildtype=release'],
  meson_version: '>=1.1',
)

# Headers only: the plugin resolves libobs symbols from the OBS process that loads it.
libobs_dep = subproject('obs-studio').get_variable('libobs_dep')

c_args = ['-DPLUGIN_VERSION="@0@"'.format(get_option('plugin_version'))]
link_args = []
link_with = []
deps = [libobs_dep, dependency('threads')]

if host_machine.system() == 'windows'
  # Windows needs an import library for obs.dll; obs.def lists what the plugin uses.
  dlltool = find_program('x86_64-w64-mingw32-dlltool')
  link_with += custom_target(
    'obs-import-lib',
    input: 'obs.def',
    output: 'libobs.dll.a',
    command: [dlltool, '--input-def', '@INPUT@', '--output-lib', '@OUTPUT@', '--dllname', 'obs.dll'],
  )
  deps += meson.get_compiler('c').find_library('ws2_32')
  # One self-contained DLL, without libwinpthread or libgcc beside it.
  link_args += ['-static']
elif host_machine.system() == 'darwin'
  c_args += ['-mmacosx-version-min=11.0']
  link_args += ['-mmacosx-version-min=11.0']
endif

shared_module(
  'obs-streamplace',
  'src/plugin-main.c',
  'src/streamplace-output.c',
  'src/fmp4.c',
  'src/http.c',
  name_prefix: '',
  c_args: c_args,
  link_args: link_args,
  link_with: link_with,
  dependencies: deps,
  gnu_symbol_visibility: 'hidden',
)
