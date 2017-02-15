#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

# Realpath polyfill!
if ! which 'realpath' &> /dev/null; then
  realpath() {
    (cd "$(dirname "$1")" && pwd)
  }
  export -f realpath
fi

DOCKER_PREFIX=${DOCKER_PREFIX:-docker.io/streamplace}
THIS_IS_CI="${THIS_IS_CI:-}"

# Add node_modules to path
export PATH="$PATH:$ROOT/node_modules/.bin"

# If we're in a package, this gets our name, so...
my_dir="$(basename $(realpath .))"
if [[ "$(pwd)" == "$ROOT/packages/$my_dir" ]]; then
  PACKAGE_NAME="$my_dir"
fi


# Use jq to alter the specified JSON blob
function tweak() {
  json="$1"
  key="$2"
  value="$3"
  echo "$json" | jq -r "$key = \"$value\""
}

# Easy reusable confirmation dialog
function confirm() {
  read -p "$1 " -n 1 -r
  echo    # (optional) move to a new line
  if [[ ! $REPLY =~ ^[Yy]$ ]]
  then
    echo "Exiting..."
    exit 0
  fi
}

# Lots of logging past this point...

# Colors!
RESTORE='\033[0m'

RED='\033[00;31m'
GREEN='\033[00;32m'
YELLOW='\033[00;33m'
BLUE='\033[00;34m'
PURPLE='\033[00;35m'
CYAN='\033[00;36m'
LIGHTGRAY='\033[00;37m'

LRED='\033[01;31m'
LGREEN='\033[01;32m'
LYELLOW='\033[01;33m'
LBLUE='\033[01;34m'
LPURPLE='\033[01;35m'
LCYAN='\033[01;36m'
WHITE='\033[01;37m'

function test_colors(){

  echo -e "${GREEN}Hello ${CYAN}THERE${RESTORE} Restored here ${LCYAN}HELLO again ${RED} Red socks aren't sexy ${BLUE} neither are blue ${RESTORE} "

}

function info() {
  echo -en "${CYAN}"
  echo -en "[info] $1"
  echo -e "${RESTORE}"
}

function log() {
  echo -en "${GREEN}"
  echo -en "$1"
  echo -e "${RESTORE}"
}

function fixOrErr() {
  echo -en "${RED}"
  echo -en "$1 "
  if [[ ${FIX_OR_ERR:-FIX} == "ERR" ]]; then
    exit 1
  else
    echo -en "${GREEN}"
    log "Fixing..."
  fi
  echo -e "${RESTORE}"
}
