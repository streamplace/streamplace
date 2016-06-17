<p align="center">
  <a href="https://stream.kitchen/">
    <img alt="Stream Kitchen" src="https://cloud.githubusercontent.com/assets/257909/15797920/2446bcae-29dc-11e6-8ea7-fbde5f56f408.png" width="150">
  </a>
</p>

<h3 align="center">Stream Kitchen</h3>

<p align="center">
  <a href="https://travis-ci.org/streamkitchen/streamkitchen">
    <img src="https://travis-ci.org/streamkitchen/streamkitchen.svg?branch=master">
  </a>
  <a href="https://slack.stream.kitchen/">
    <img src="https://slack.stream.kitchen/badge.svg">
  </a>
</p>

<p align="center">
  A toolkit for compositing live video streams in the cloud.
</p>

<p align="center">
  <a href="https://www.kickstarter.com/projects/338091149/stream-kitchen">Check out our Kickstarter video for examples!</a>
</p>

Installation Instructions
-------------------------

**Prerequisites**
* docker 1.10+
* docker-compose 1.7+

**docker-machine**

If you're not running Linux, everything will work just fine using docker-machine, but you should
consider using one of the drivers that actually spawns a remote server for you, like `digitalocean`
or `amazonec2`. The `virtualbox` driver, or any other local VM, will likely not be fast enough to
transcode live video. Here's a command to summon a moderately-sized EC2 instance:

```
docker-machine create \
  --driver amazonec2 \
  --amazonec2-instance-type c4.xlarge \
  default
```

**Clone the Repo**

```
git clone git@github.com/streamkitchen/streamkitchen
cd streamkitchen
run/docker-compose.sh
```

If that all works well, you're good to go! The web interface is at `http://localhost:5000`, and
you'll be able to handle incoming RTMP connections at `rtmp://localhost/stream/{stream_key}`.
