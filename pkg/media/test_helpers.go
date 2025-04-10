package media

import (
	"path/filepath"
	"runtime"
)

func GetFixture(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "test", "fixtures", name)
}
