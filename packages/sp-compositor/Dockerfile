FROM quay.io/streamplace/sp-builder as builder
FROM quay.io/streamplace/sp-ffmpeg

WORKDIR /app
RUN apt-get update

# presumably chromium and electron need the same things...?
RUN apt-get install -y chromium-browser libgconf-2-4 libgtk2.0-0 xvfb libgles1-mesa unclutter

ADD package.json /app/package.json
RUN npm install

ENV ELECTRON_ENABLE_LOGGING true
ENV XVFB_SCREENSIZE 1920x1080x16
ENV DISPLAY :99

ADD run.sh /app/run.sh
COPY --from=builder /app/dist /app/dist

ENTRYPOINT ["/app/run.sh"]
