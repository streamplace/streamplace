# @streamplace/config-react-native-webrtc

This npm module contains code to install the Streamplace-optimized version of
react-native-webrtc.

To use:
`[yarn/pnpm/npm] add @streamplace/config-react-native-webrtc react-native-webrtc`

And add to the `plugins` array in your `app.config.ts`:

```tsx
export default {
  expo: {
    plugins: ["@streamplace/config-react-native-webrtc"],
  },
};
```
