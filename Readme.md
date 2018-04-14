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

## Development

### Client & Server

First:

```
yarn install
```

### Server

You'll need a local Kubernetes cluster. Kubernetes for Docker for Mac and minikube should both work.

You'll also need helm on your PATH.

To boot everything up initially:

```
npm run env:dev
npm run server
```

To view output logs on local Kibana:

```
npm run server:logs
```

Then, after you've made some local changes and you want to deploy them to the cluster without
rebuilding everything:

```
npm run server:apply [packages you want to deploy]
```

### Client Development

If you want to develop the client against the staging server, that's fine, just run:

```
npm run env:next
```

Then, to boot up the web, Electron, and React Native client apps:

```
npm run client
```

Electron, Web, and the iOS Simulator should work out of the box. To get Android emulators working
you'll probably need to forward port 80 on the device with `adb`.

```
adb root
adb reverse tcp:80 tcp:80
```

### Development Environment Known Issues

1.  Sometimes stuff just doesn't come up.

    * Usually it's `sp-api-server` or `sp-schema` for whatever reason. Usually it can be resolved with a `kubectl get pods` and `kubectl delete pod [malfunctioning-pod-name]`.

1.  Sometimes `sp-frontend` takes a ton of time to compile and makes the computer's fan spin like crazy.

    * Yup. This one is currently a mystery to me. Deleting the `sp-frontend` pod usually speeds it back up. If we can track this one down, potentially we can file a bug with Docker for Mac or create-react-app.
