#!/usr/bin/env python3
"""Finalize Windows desktop signatures before publishing Squirrel artifacts."""

from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import zipfile


ROOT = Path(__file__).resolve().parents[2]


def needs_signature(file, length):
    if file.read(2) != b"MZ":
        return None
    file.seek(0x3C)
    offset = struct.unpack("<I", file.read(4))[0]
    file.seek(offset)
    if file.read(4) != b"PE\0\0":
        return None
    coff = file.read(20)
    optional_size = struct.unpack_from("<H", coff, 16)[0]
    optional = file.read(optional_size)
    magic = struct.unpack_from("<H", optional)[0]
    if magic not in (0x10B, 0x20B):
        raise ValueError(f"Unsupported PE optional header: {magic:#x}")
    directories = 96 if magic == 0x10B else 112
    count = struct.unpack_from("<I", optional, directories - 4)[0]
    if count <= 4:
        return True
    offset, size = struct.unpack_from("<II", optional, directories + 4 * 8)
    if offset == size == 0:
        return True
    if not offset or size < 8 or offset + size > length:
        raise ValueError("Invalid PE certificate table")
    return False


def sign(files):
    if files:
        subprocess.run(
            ["bash", str(ROOT / "hack/sign-windows.sh"), *map(str, files)],
            check=True,
        )


def sign_package(directory):
    files = []
    for path in directory.rglob("*"):
        if path.is_file():
            with path.open("rb") as file:
                if path == directory / "Streamplace.exe" or needs_signature(file, path.stat().st_size):
                    files.append(path)
    sign(files)


def finalize_squirrel(package, setup, vendor, icon):
    # The Mono maker adds the execution stub and icon-edited Squirrel.exe after
    # Forge's postPackage hook. Replace just the entries that need signing.
    with tempfile.TemporaryDirectory(prefix="streamplace-sign-") as temporary:
        work = Path(temporary)
        files = []
        updater = None
        with zipfile.ZipFile(package) as archive:
            for entry in archive.infolist():
                if entry.is_dir():
                    continue
                is_updater = entry.filename.lower() == "lib/net45/squirrel.exe"
                with archive.open(entry) as file:
                    if not is_updater and not needs_signature(file, entry.file_size):
                        continue
                path = Path(archive.extract(entry, work))
                if path.relative_to(work).as_posix() != entry.filename:
                    raise ValueError(f"Invalid Squirrel package path: {entry.filename}")
                files.append(path)
                if is_updater:
                    updater = path
        if updater is None:
            raise ValueError("Missing Squirrel updater in full package")
        sign(files)
        subprocess.run(["zip", "-q", package, *(p.relative_to(work) for p in files)], cwd=work, check=True)
        sha1 = subprocess.check_output(["sha1sum", package]).split()[0].decode("ascii").upper()
        release = f"{sha1} {package.name} {package.stat().st_size}\n"
        signed_embedded = work / "setup.zip"
        with zipfile.ZipFile(signed_embedded, "w") as archive:
            archive.write(package, package.name)  # already deflated
            archive.writestr("RELEASES", release)
            archive.write(updater, "Update.exe", zipfile.ZIP_DEFLATED)
            archive.write(icon, "setupIcon.ico", zipfile.ZIP_DEFLATED)
            archive.write(vendor.parent / "resources/install-spinner.gif", "background.gif", zipfile.ZIP_DEFLATED)
        command = [str(vendor / "WriteZipToSetup.exe"), str(setup), str(signed_embedded)]
        if sys.platform != "win32":
            command.insert(0, "wine")
        subprocess.run(command, check=True)
        # Resource replacement changes Setup.exe. Always sign it last, even if
        # the input happened to contain a certificate table.
        sign([setup])
        package.with_name("RELEASES").write_text(release, encoding="utf-8")


if __name__ == "__main__":
    if len(sys.argv) >= 3 and sys.argv[1] == "package":
        for directory in sys.argv[2:]:
            sign_package(Path(directory).resolve())
    elif len(sys.argv) == 6 and sys.argv[1] == "squirrel":
        finalize_squirrel(*(Path(value).resolve() for value in sys.argv[2:]))
    else:
        raise SystemExit(
            "Usage: sign-windows.py package <directory> [...] | "
            "squirrel <full.nupkg> <Setup.exe> <electron-winstaller vendor> <setup icon>"
        )
