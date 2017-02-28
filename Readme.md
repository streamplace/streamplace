<p align="center">
  <a href="https://stream.kitchen/">
    <img alt="streamplace" src="https://cloud.githubusercontent.com/assets/257909/22085092/7e32de3c-dd87-11e6-8209-26176f852912.png" width="700">
  </a>
</p>

<p align="center">
  <a href="https://travis-ci.org/streamplace/streamplace">
    <img src="https://travis-ci.org/streamplace/streamplace.svg?branch=master">
  </a>
  <a href="https://slack.stream.place/">
    <img src="https://slack.stream.place/badge.svg">
  </a>
</p>

<p align="center">
  An open-source compositor and CMS for live video.
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
