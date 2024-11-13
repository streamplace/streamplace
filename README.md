# Aquareum

## The Video Layer for Everything

You'll need Node, Yarn, Meson, Ninja, and Go.

**Single-command build:**

```
make
```

**Dev Setup**

```
yarn install
yarn run build
```

**Node Development**

```
make node
./build-darwin-arm64/aquareum

// or

./build-linux-amd64/aquareum
```

**Web Development**

```
yarn run app start
```

**App Development**

```
cd js/app

# iOS
npx expo run:ios -d "Your iPhone"

# Android
npx expo run:android

# Web http://localhost:38081
npx expo start
```

You can also boot the other platforms directly from the bundler once it starts.

**Desktop Development**

```
yarn run desktop start
```

If you need to set up an admin account key for using your local desktop app
against your local node, run:

```
node util/generate-dev-admin-key.mjs
```

Adding those two environment variables to your shell will give your desktop app
administrative access over your local Aquareum node.
