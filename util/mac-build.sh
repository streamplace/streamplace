#!/bin/bash

set -euo pipefail
set -x

# setup environment
export LANG=en_US.UTF-8
# echo 'admin' | sudo -S umount "/Volumes/My Shared Files"
# mount_virtiofs com.apple.virtio-fs.automount ~/build
mkdir ~/build 
cd ~/build
source '/Volumes/My Shared Files/signing/mac-codesigning.env'
git clone '/Volumes/My Shared Files/streamplace'
eval "$(/opt/homebrew/bin/brew shellenv)"
brew update --force --quiet
cd ~/build/streamplace
brew install ninja go openssl@3 cocoapods git && go version
sudo gem install --user-install xcpretty
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs > rustup.sh && bash rustup.sh -y && rm rustup.sh
export PATH="$PATH:$HOME/.cargo/bin:$(find $HOME/.gem/ruby -type d -name bin -maxdepth 2)"
export PATH="/opt/homebrew/opt/m4/bin:$PATH"
brew install python@3.11 node
python3.11 -m pip install virtualenv
python3.11 -m virtualenv ~/venv
source ~/venv/bin/activate
pip3 install meson
make node-all-platforms-macos -j16
