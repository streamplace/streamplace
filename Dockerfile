# ==============
# DO NOT EDIT!!!
# ==============
#
# This file is generated automatically by run/generate-dockerfile.js.
# Edit that, not this.
FROM stream.place/sp-node

RUN npm install -g yarn lerna
WORKDIR /app
ENV NODE_ENV production
# add all package.json files
ADD packages/sp-api-server/package.json /app/packages/sp-api-server/package.json
ADD packages/sp-app/package.json /app/packages/sp-app/package.json
ADD packages/sp-auth-frontend/package.json /app/packages/sp-auth-frontend/package.json
ADD packages/sp-broadcaster/package.json /app/packages/sp-broadcaster/package.json
ADD packages/sp-builder/package.json /app/packages/sp-builder/package.json
ADD packages/sp-builder-static/package.json /app/packages/sp-builder-static/package.json
ADD packages/sp-channel-manager/package.json /app/packages/sp-channel-manager/package.json
ADD packages/sp-client/package.json /app/packages/sp-client/package.json
ADD packages/sp-components/package.json /app/packages/sp-components/package.json
ADD packages/sp-compositor/package.json /app/packages/sp-compositor/package.json
ADD packages/sp-configuration/package.json /app/packages/sp-configuration/package.json
ADD packages/sp-conformance/package.json /app/packages/sp-conformance/package.json
ADD packages/sp-coturn/package.json /app/packages/sp-coturn/package.json
ADD packages/sp-default/package.json /app/packages/sp-default/package.json
ADD packages/sp-dev/package.json /app/packages/sp-dev/package.json
ADD packages/sp-ffmpeg/package.json /app/packages/sp-ffmpeg/package.json
ADD packages/sp-file-server/package.json /app/packages/sp-file-server/package.json
ADD packages/sp-frontend/package.json /app/packages/sp-frontend/package.json
ADD packages/sp-ingress/package.json /app/packages/sp-ingress/package.json
ADD packages/sp-kube-lego/package.json /app/packages/sp-kube-lego/package.json
ADD packages/sp-logs/package.json /app/packages/sp-logs/package.json
ADD packages/sp-media-player/package.json /app/packages/sp-media-player/package.json
ADD packages/sp-native/package.json /app/packages/sp-native/package.json
ADD packages/sp-node/package.json /app/packages/sp-node/package.json
ADD packages/sp-overlay/package.json /app/packages/sp-overlay/package.json
ADD packages/sp-peer-stream/package.json /app/packages/sp-peer-stream/package.json
ADD packages/sp-plugin-auth/package.json /app/packages/sp-plugin-auth/package.json
ADD packages/sp-plugin-core/package.json /app/packages/sp-plugin-core/package.json
ADD packages/sp-raspberrypi/package.json /app/packages/sp-raspberrypi/package.json
ADD packages/sp-redirects/package.json /app/packages/sp-redirects/package.json
ADD packages/sp-resource/package.json /app/packages/sp-resource/package.json
ADD packages/sp-rethinkdb/package.json /app/packages/sp-rethinkdb/package.json
ADD packages/sp-rtmp-nginx/package.json /app/packages/sp-rtmp-nginx/package.json
ADD packages/sp-rtmp-server/package.json /app/packages/sp-rtmp-server/package.json
ADD packages/sp-schema/package.json /app/packages/sp-schema/package.json
ADD packages/sp-static/package.json /app/packages/sp-static/package.json
ADD packages/sp-streams/package.json /app/packages/sp-streams/package.json
ADD packages/sp-styles/package.json /app/packages/sp-styles/package.json
ADD packages/sp-uploader/package.json /app/packages/sp-uploader/package.json
ADD packages/sp-utils/package.json /app/packages/sp-utils/package.json
ADD packages/streamplace/package.json /app/packages/streamplace/package.json
ADD packages/streamplace-ui/package.json /app/packages/streamplace-ui/package.json
# build everyone's production dependencies into one big blob
ADD package.json /app/package.json
RUN find . -maxdepth 3 -type f -name package.json -exec sed -i s/devDependencies/devDependenciesRemoved/ {} \; && yarn install --prod && yarn cache clean
