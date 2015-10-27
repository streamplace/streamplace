
Package.describe({
  summary: 'bellamie do thing',
  version: '0.0.0',
  name: 'streamplace:bellamie',
  git: ''
});

Npm.depends({

});

Package.on_use(function (api) {
  api.use('mongo');

  api.add_files([
    'model.js',
    'broadcasts.js',
  ], 'server');
});
