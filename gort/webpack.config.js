module.exports = {
  entry: "./index.jsx",
  output: {
    filename: "bundle.js", //this is the default name, so you can skip it
    //at this directory our bundle file will be available
    //make sure port 8090 is used when launching webpack-dev-server
    publicPath: "http://drumstick.iame.li:5050/assets/"
  },
  module: {
    loaders: [{
      test: /\.jsx?$/,
      exclude: /(node_modules|bower_components)/,
      loader: "babel"
    }, {
      test: /\.scss$/,
      loaders: ["style", "css?modules", "sass"]
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
    extensions: [".js", ".jsx"]
  }
};
