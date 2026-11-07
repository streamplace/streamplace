project('obs-studio', 'c', version: '30.2.3')

libobs_dep = declare_dependency(include_directories: include_directories('libobs', is_system: true))
frontend_api_dep = declare_dependency(
  include_directories: include_directories('UI/obs-frontend-api', is_system: true),
  dependencies: libobs_dep,
)
