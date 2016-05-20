Dandiprat Apps


Dev Ports
---------

* 1934: Shoko RTMP
* 4949: Swagger Editor
* 8100: Bellamie
* 8200: Shoko HLS


Rough Style Guide
-----------------

Javascript
  * Needs to pass ESLint. Run `npm run lint` in the root directory to check.
  * Try and use both `let` and `const` in whatever way makes the code most understandable.
  * We've opted into two features that will be likely landing in ES2017:
    * [the function bind operator ::](https://github.com/zenparsing/es-function-bind)
    * [object rest/spread properties ({...obj})][https://github.com/sebmarkbage/ecmascript-rest-spread]
