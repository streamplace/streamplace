FROM quay.io/streamplace/sp-node

WORKDIR /app
RUN apt-get update

# presumably chromium and electron need the same things...?
RUN apt-get install -y chromium-browser libgconf-2-4 libgtk2.0-0 xvfb

ADD package.json /app/package.json
RUN npm install

ENV ELECTRON_ENABLE_LOGGING true
ENV XVFB_SCREENSIZE 1920x1080x16
ENV DISPLAY :99

ADD dist /app/dist
ADD run.sh /app/run.sh

CMD /app/run.sh
