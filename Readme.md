<p align="center">
  <a href="https://stream.kitchen/">
    <img alt="streamplace" src="https://cloud.githubusercontent.com/assets/257909/22085092/7e32de3c-dd87-11e6-8209-26176f852912.png" width="700">
  </a>
</p>

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

Currently WIP as we set up an easy way to do local Kubernetes. For now hit up [Slack](https://slack.stream.kitchen/) and I'll walk you through it.


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
