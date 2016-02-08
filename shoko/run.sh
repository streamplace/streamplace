#!/bin/bash
docker run -d -p 8200:80 -p 1934:1934 --name=shoko gcr.io/stream-kitchen/shoko
