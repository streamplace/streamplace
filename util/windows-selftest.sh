#!/bin/bash

set -euo pipefail
set -x

verify_signatures() {
  if [ "${SP_WINDOWS_SIGN:-0}" = 1 ]; then
    SIGNATURE_PATH="$(cygpath -aw "$1")" powershell -NoProfile -Command '
      $ErrorActionPreference = "Stop"
      Get-ChildItem -LiteralPath $env:SIGNATURE_PATH -File -Recurse |
        Where-Object { $_.Extension -in ".exe", ".dll", ".node" } |
        ForEach-Object {
          $sig = Get-AuthenticodeSignature -LiteralPath $_.FullName
          if ($sig.Status -ne "Valid") { throw "$($_.FullName): $($sig.StatusMessage)" }
        }'
  fi
}

export AQD_NO_UPDATE=true
curl --fail -L -o streamplace-desktop.exe "$1"
verify_signatures streamplace-desktop.exe
./streamplace-desktop.exe
powershell -Command "Stop-Process -Name Streamplace"
cd "$LOCALAPPDATA/streamplace_desktop"
verify_signatures .
cd app-*
test -f resources/obs-streamplace/bin/64bit/obs-streamplace.dll
./resources/streamplace.exe --version
./Streamplace.exe -- --self-test
