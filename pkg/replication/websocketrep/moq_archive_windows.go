package websocketrep

import "golang.org/x/sys/windows"

// Windows does not support fsync on a directory handle. MoveFileEx with
// WRITE_THROUGH commits the rename after the temporary file itself was flushed.
func commitMoqArchive(from, to string) error {
	source, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	destination, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(source, destination, windows.MOVEFILE_WRITE_THROUGH|windows.MOVEFILE_REPLACE_EXISTING)
}
