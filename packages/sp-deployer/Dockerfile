
FROM streamplace/sp-node

RUN npm install -g --unsafe-perm helm-cli \
  && npm install -g --unsafe-perm kubectl-cli \
  && helm init --client-only

RUN helm repo add streamplace https://charts.stream.place
