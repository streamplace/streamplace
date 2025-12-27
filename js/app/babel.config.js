const lucideTransformPlugin = require("./babel-plugin-lucide-transform");

module.exports = (api) => {
  api.cache(true);
  return {
    presets: [["babel-preset-expo", { jsxRuntime: "automatic" }]],
    plugins: [
      lucideTransformPlugin,
      // NOTE: this is only necessary if you are using reanimated for animations
      "react-native-reanimated/plugin",
    ],
  };
};
