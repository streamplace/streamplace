<p align="center">
  <a href="https://stream.place/">
    <img alt="streamplace" src="https://crap.stream.place/icon.svg" width="200">
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

### FAQ

**How do I use it?**

You don't. Not yet. We're still making it.

**Okay but I know some things about Javascript, can I boot up a development version?**

Maybe. You'll need:

* node 6+
* [Docker for Mac](https://www.docker.com/products/docker). (Untested on Linux, but I think it'll
  work.)

```
git clone git@github.com:streamplace/streamplace.git
cd streamplace
npm install
npm run start
```

Follow the prompts. The first run will take a long time, as it has to build all of Streamplace's
Docker images from scratch.

And then you'll have a working dev version of Streamplace on your computer. I mean, it won't *do* anything yet, but you'll have it.

### Development Environment Known Issues

1. Sometimes stuff just doesn't come up.

   * Usually it's `sp-api-server` or `sp-schema` for whatever reason. Usually it can be resolved with a `kubectl get pods` and `kubectl delete pod [malfunctioning-pod-name]`.

1. Sometimes `sp-frontend` takes a ton of time to compile and makes the computer's fan spin like crazy.

   * Yup. This one is currently a mystery to me. Deleting the `sp-frontend` pod usually speeds it back up. If we can track this one down, potentially we can file a bug with Docker for Mac or create-react-app.
