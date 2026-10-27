const lucideTransformPlugin = require("./babel-plugin-lucide-transform");
const walletLibsPlugin = require("./babel-plugin-wallet-libs");

module.exports = (api) => {
  api.cache(true);
  return {
    presets: [["babel-preset-expo", { jsxRuntime: "automatic" }]],
    plugins: [
      lucideTransformPlugin,
      // NOTE: this is only necessary if you are using reanimated for animations
      "react-native-reanimated/plugin",
    ],
    overrides: [
      {
        // Sign in with Ethereum's libraries; see babel-plugin-wallet-libs.js
        test: /node_modules[\\/](@reown|@walletconnect|@wagmi|wagmi|viem|ox|valtio|zustand)[\\/]/,
        plugins: [walletLibsPlugin],
      },
    ],
  };
};
