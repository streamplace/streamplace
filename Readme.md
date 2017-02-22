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

Maybe. You'll need VirtualBox and Keybase. And you'll have to tell Eli your Keybase username on Slack. Once you've done that:

```
npm install -g minikube-cli
minikube start
git clone git@github.com:streamplace/streamplace.git
cd streamplace
npm install
npm run bootstrap
npm run docker-build # this step takes a LONG time, ~30min
npm run helm-dev
npm start
```

And then you'll have a working dev version of Streamplace on your computer. I mean, it won't *do* anything yet, but you'll have it.
