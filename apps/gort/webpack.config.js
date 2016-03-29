
/*eslint-disable no-var */
var path = require("path");

module.exports = {
  context: __dirname,
  entry: "./src/index",
  output: {
    filename: "bundle.js", //this is the default name, so you can skip it
    path: "dist",
    //at this directory our bundle file will be available
    //make sure port 8090 is used when launching webpack-dev-server
    publicPath: "http://drumstick.iame.li:5050/assets/"
  },
  module: {
    loaders: [{
      test: /\.jsx?$/,
      exclude: /(node_modules|bower_components)/,
      loader: "babel",
      query: {
        presets: [
          // Weirdness here because of https://github.com/babel/babel-loader/issues/166
          require.resolve("babel-preset-react"),
          require.resolve("babel-preset-es2015")
        ],
        plugins: [
          require.resolve("babel-plugin-transform-object-rest-spread")
        ]
      }
    }, {
      test: /\.scss$/,
      loaders: ["style", "css?modules", "sass"]
    }, {
      test: /\.json$/,
      loaders: ["json"]
    }, {
      test: /\.woff(2)?(\?v=[0-9]\.[0-9]\.[0-9])?$/,
      loader: "url-loader?limit=10000&minetype=application/font-woff"
    }, {
      test: /\.(ttf|eot|svg)(\?v=[0-9]\.[0-9]\.[0-9])?$/,
      loader: "file-loader"
    }]
  },
  externals: {
    //don"t bundle the "react" npm package with our bundle.js
    //but get it from a global "React" variable
    "react": "React"
  },
  resolve: {
    root: [
      path.resolve(__dirname),
      path.resolve(".."), // So we can resolve all our other packages
    ],
    extensions: [".js", ".jsx", ".scss", "json", ""]
  },
  resolveLoader: {
    root: path.join(__dirname, "node_modules")
  }
};
