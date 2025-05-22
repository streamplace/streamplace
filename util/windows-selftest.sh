#!/bin/bash

set -euo pipefail
set -x

export AQD_NO_UPDATE=true
curl -L -o streamplace-desktop.exe "$1"
./streamplace-desktop.exe
powershell -Command "Stop-Process -Name Streamplace"
cd "$LOCALAPPDATA/streamplace_desktop"
cd app-*
./Streamplace.exe -- --self-test
