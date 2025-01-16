#!/bin/bash

set -euo pipefail
set -x

curl -L -o streamplace-desktop.exe "$1"
./streamplace-desktop.exe
powershell -Command "Stop-Process -Name Streamplace"
cd /c/Users/admin/AppData/Local/streamplace_desktop
cd app-*
./Streamplace.exe -- --self-test
