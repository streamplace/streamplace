<p align="center">
  <a href="https://stream.place/">
    <img alt="streamplace" src="https://various-rando-files.stream.place/icon.svg" width="200">
  </a>
</p>

<h1 align="center">Streamplace</h1>

<p align="center">
  <a href="https://travis-ci.org/streamplace/streamplace">
    <img src="https://img.shields.io/travis/streamplace/streamplace/master.svg?label=Travis">
  </a>
  <a href="https://circleci.com/gh/streamplace/streamplace">
    <img src="https://img.shields.io/circleci/project/github/streamplace/streamplace/master.svg?label=CircleCI">
  </a>
  <a href="https://slack.stream.place/">
    <img src="https://slack.stream.place/badge.svg">
  </a>
</p>

<p align="center">
  An open-source compositor and CMS for live video!
</p>

<p align="center">
  <a href="https://www.kickstarter.com/projects/338091149/stream-kitchen">Check out our Kickstarter video for examples!</a>
</p>

### Client Development

To develop all the Streamplace clients at the same time, just run:

```
yarn install
npm run dev-client
```

To get Android emulators working you'll probably need to forward some ports from the device with `adb`.

```
adb root
adb reverse tcp:80 tcp:80
adb reverse tcp:19000 tcp:19000
```

### Development Environment Known Issues

1.  Sometimes stuff just doesn't come up.

    * Usually it's `sp-api-server` or `sp-schema` for whatever reason. Usually it can be resolved with a `kubectl get pods` and `kubectl delete pod [malfunctioning-pod-name]`.

1.  Sometimes `sp-frontend` takes a ton of time to compile and makes the computer's fan spin like crazy.

    * Yup. This one is currently a mystery to me. Deleting the `sp-frontend` pod usually speeds it back up. If we can track this one down, potentially we can file a bug with Docker for Mac or create-react-app.
