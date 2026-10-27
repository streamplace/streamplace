// The entry points Forge's webpack plugin defines (DefinePlugin), by the
// bundle's location in development or production. DefinePlugin substitutes
// only where these names are used, so reference them directly: a module
// that re-exports one exports undefined.
declare const MAIN_WINDOW_WEBPACK_ENTRY: string;
declare const MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY: string;
