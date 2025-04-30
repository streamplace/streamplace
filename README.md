# Streamplace

## Sponsorship

![image](https://github.com/user-attachments/assets/1e3e13ab-ef6f-43dd-9d8b-15645adbd695)

Streamplace was generously funded by the Livepeer Treasury as part of their
mission to build the world's open video infrastructure. Check out more at
[livepeer.org](https://www.livepeer.org/)!

## The Video Layer for Everything

You'll need Node, Yarn, Meson, Ninja, and Go 1.24.

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
that happen, you can set the `SP_APP_SCHEME` environment variable to the scheme
you want to use; it must match reverse of the URL of a publicly accessible
server with an HTTPS cert. So, if you're exposing your app on
streamplace.example.com, you could run the app with

```
export SP_APP_SCHEME=com.example.streamplace
yarn run app ios
```

**Streaming during development**

You can stream OBS into your local Streamplace node the same way you would in a
production setting. OBS can be a bit resource-intensive, so you can also run a
WHIP-enabled FFMPEG on the command line. That command might look like this:

```
docker run -it --rm \
  -v $HOME/testvids:/testvids \
  docker.io/ggtoms/ffmpeg-webrtc \
  /usr/local/ffmpeg-webrtc/ffmpeg -re -i \
  [EXAMPLE-VIDEO.mp4] \
  -c copy -f whip \
  http://[LAN IP ADDRESS]:38080/api/ingest/webrtc/[STREAM KEY]
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
