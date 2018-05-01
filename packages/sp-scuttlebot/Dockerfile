FROM stream.place/streamplace
RUN ln -s /data /root/.ssb
WORKDIR /app/node_modules/sp-scuttlebot
ENV PATH $PATH:/app/node_modules/.bin
CMD scuttlebot
