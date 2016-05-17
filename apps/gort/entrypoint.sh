#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# We need to dynamically configure our API server at container runtime, thus this file.
# Usage: ./entrypoint.sh [location of index.html]

API_SERVER_URL=${API_SERVER_URL:-}
if [[ "$API_SERVER_URL" == "" ]]; then
  echo "Missing required environment variable: API_SERVER_URL"
  exit 1
fi

cat > "$1" << EOF
<!DOCTYPE html>
<html>
<head>
  <title>Stream Kitchen</title>
  <link rel="icon" href="https://s3-us-west-2.amazonaws.com/stream.kitchen/favicon.png">
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/normalize/3.0.3/normalize.min.css">
  <script src="https://cdnjs.cloudflare.com/ajax/libs/react/15.0.2/react-with-addons.min.js"></script>
  <script>
    window.SK_PARAMS = {
      API_SERVER_URL: "$API_SERVER_URL"
    };
  </script>
</head>
<body>
  <main></main>
  <script type="text/javascript" src="/app/dist/bundle.js"></script>
</body>
</html>
EOF

exec nginx
