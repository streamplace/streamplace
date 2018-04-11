FROM stream.place/sp-ffmpeg

WORKDIR /app
RUN apt-get update

# presumably chromium and electron need the same things...?
RUN apt-get install -y chromium-browser libgconf-2-4 libgtk2.0-0 xvfb libgles1-mesa unclutter

ENV ELECTRON_ENABLE_LOGGING true
ENV XVFB_SCREENSIZE 1920x1080x16
ENV DISPLAY :99

COPY --from=stream.place/streamplace /app /app
ADD run.sh /app/node_modules/sp-compositor/run.sh
ENTRYPOINT ["/app/node_modules/sp-compositor/run.sh"]
