module.exports = {
  context: __dirname,
  entry: "./src/sp-styles.scss",
  output: {
    filename: "sp-styles.js",
    path: "dist",
    libraryTarget: "umd"
  },
  module: {
    loaders: [
      {
        test: /\.jsx?$/,
        exclude: /(node_modules|bower_components)/,
        loader: "babel-loader",
        query: {
          presets: ["streamplace"]
        }
      },
      {
        test: /\.scss$/,
        loaders: ["style-loader", "css-loader?modules", "sass-loader"]
      }
    ]
  }
};
