Dandiprat Apps


Dev Ports
---------

* 1934: Shoko RTMP
* 4949: Swagger Editor
* 8100: Bellamie
* 8200: Shoko HLS


Useful ffmpeg commands
----------------------

Stream an image:

ffmpeg \
  -loop 1 \
  -re \
  -framerate 30 \
  -i sk_1080p.png \
  -f lavfi \
  -i anullsrc \
  -c:v libx264 \
  -c:a aac \
  -strict experimental \
  -b:a 192k \
  -f flv \
  rtmp://drumstick.iame.li:1934/stream/image
