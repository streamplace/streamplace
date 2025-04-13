FROM ubuntu:24.04

# testing container that bundles a big video file. it downloads the video first
# for efficient docker layer caching

RUN apt-get update && apt-get install -y ca-certificates curl
RUN curl https://storage.googleapis.com/streamplace-crap/BigBuckBunny_1sGOP_4kp60_NoBframes.mp4 -o /bunny.mp4
COPY bunny.sh /bunny.sh
RUN chmod +x /bunny.sh

CMD ["/bunny.sh"]
