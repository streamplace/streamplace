<p align="center">
  <a href="https://stream.kitchen/">
    <img alt="Stream Kitchen" src="https://cloud.githubusercontent.com/assets/257909/15797920/2446bcae-29dc-11e6-8ea7-fbde5f56f408.png" width="267"><br>
  </a>
</p>

<h3 align="center">Stream Kitchen</h3>

<p align="center">
  A toolkit for compositing live video streams in the cloud.
</p>

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
    * [the function bind operator `::`](https://github.com/zenparsing/es-function-bind)
    * [object rest/spread properties `{...obj}`](https://github.com/sebmarkbage/ecmascript-rest-spread)
