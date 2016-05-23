#!/bin/bash

node /app/dist/app.js

# When we're done, obliterate our pod. This is a hack.
curl -v --cacert /var/run/secrets/kubernetes.io/serviceaccount/ca.crt -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" -X DELETE https://kubernetes/api/v1/namespaces/default/pods/$(hostname)
