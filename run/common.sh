#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"

# Realpath polyfill!
function realpath() {
  # resolve="$1"
  # realDir="$(cd "$(dirname "$1")" && pwd)"
  # if [[ -f "$resolve" ]]; then
  #   echo -n "$realDir/$(basename $resolve)"
  # elif [[ -d "$resolve" ]]; then
  #   echo -n "$realDir"
  # else
  #   echo -n "realpath failed, $resolve does not exist" >&2
  #   return 1
  # fi
  for file in "$@"; do
    while [ -h "$file" ]; do
      l=`readlink $file`
      case "$l" in
        /*) file="$l";;
        *) file=`dirname "$file"`/"$l"
      esac
    done
    #echo $file
    python -c "import os,sys; print os.path.abspath(sys.argv[1])" "$file"
  done
}
export -f realpath

function base64() {
  node "$ROOT/run/base64.js" $*
}
export -f base64

DOCKER_PREFIX=${DOCKER_PREFIX:-quay.io/streamplace}
export LOCAL_DEV="${LOCAL_DEV:-}"

export REPO_VERSION="$(node "$ROOT/run/repo-version.js")"
# ask lerna nicely to update all of our package.json files

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

if ! which jq > /dev/null; then
  function jq() {
    docker run --rm -i -e LOGSPOUT=ignore pinterb/jq "$@"
  }
fi

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
    echo -e ""
    exit 1
  else
    echo -en "${GREEN}"
    log "Fixing..."
  fi
  echo -e "${RESTORE}"
}

if [[ ! -f /var/run/docker.sock ]]; then
  if [[ "${TRAVIS_BRANCH:-}" == "" ]]; then
    if ! docker ps > /dev/null; then
      echo "No /var/run/docker.sock, and 'docker ps' is failing. Get Docker working first!"
      exit 1
    fi
  fi
fi

localKubeconfig="$(realpath "$ROOT/hack/local-kubeconfig")"
export KUBECONFIG="${KUBECONFIG:-$localKubeconfig}"
