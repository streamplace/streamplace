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


Components
----------

Microservices are good and Docker made them easy, so we have a lot of them.

**Apps**

| App  | Description |
| ------------- | ------------- |
|[bellamie][bellamie]| The SK API server, utilizing [RethinkDB][rethink] and [Socket.io][sio]. |
|[pipeland][pipeland]| The container for a single SK vertex, wrapping [FFmpeg][ffmpeg]. |
|[gort][gort]| Front-end user interface for SK, utilizing [React][react]. |
|[shoko][shoko]| RTMP server, uses [nginx-rtmp](https://github.com/arut/nginx-rtmp-module). |
|[sk-time][sk-time]| Provides an accurate clock for syncing at [time.stream.kitchen][time]. |
|[broadcast-scheduler][b-s]| Monitors active broadcasts and creates vertices and arcs as needed. |
|[vertex-scheduler][v-s]| Creates Docker containers/Kubernetes pods for existing vertices. |

[bellamie]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/bellamie
[rethink]: https://www.rethinkdb.com/
[sio]: http://socket.io/
[pipeland]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/pipeland
[ffmpeg]: https://ffmpeg.org/
[gort]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/gort
[shoko]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/shoko
[sk-time]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/sk-time
[b-s]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/broadcast-scheduler
[v-s]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/vertex-scheduler
[react]: https://facebook.github.io/react/
[time]: https://time.stream.kitchen/

**Libraries**

| Library  | Description |
| ------------- | ------------- |
|[mpeg-munger][m-m]| Provides Node streams for rewriting the timestamps (PTS) of MPEGTS streams. |
|[sk-schema][sk-schema] | *(WIP)* Swagger schema for the SK API. |
|[sk-config][sk-config] | Shared runtime configuration for all Stream Kitchen apps. |
|[sk-client][sk-client] | Javascript client for SK that works in both Node and browsers. |
|[sk-node][sk-node] | Base Docker image from which all SK Node containers inherit. |
|[sk-static][sk-node] | Docker image for hosting static SK client-side applications. |
|[twixtykit][twixtykit] | Shared SCSS styles for all SK apps and plugins. |

[m-m]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/mpeg-munger
[sk-schema]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/sk-schema
[sk-config]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/sk-config
[sk-client]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/sk-client
[sk-node]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/sk-node
[sk-node]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/sk-node
[twixtykit]: https://github.com/streamkitchen/streamkitchen/tree/master/apps/twixtykit
