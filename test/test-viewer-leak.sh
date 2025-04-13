#!/bin/bash

set -euo pipefail

GOGC=10 GOMEMLIMIT=2GiB streamplace --wide-open --no-firehose
