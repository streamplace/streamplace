# Streamplace

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
./build-darwin-arm64/streamplace

// or

./build-linux-amd64/streamplace
```

**Web Development**

```
yarn run app start
```

**App Development**

Building the Android version requires Java 17. On Ubuntu you can run
`sudo apt install openjdk-17-jdk`.

```
# iOS
yarn run app ios -d "Your iPhone"

# Android
yarn run app android

# Web http://localhost:38081
yarn run app start
```

You can also boot the other platforms directly from the bundler once it starts.

Some app development may require actual HTTPS to test the OAuth flow. To make
that happen, you can set the `AQ_APP_SCHEME` environment variable to the scheme
you want to use; it must match reverse of the URL of a publicly accessible
server with an HTTPS cert. So, if you're exposing your app on
streamplace.example.com, you could run the app with

```
export AQ_APP_SCHEME=com.example.streamplace
yarn run app ios
```

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
administrative access over your local Streamplace node.
