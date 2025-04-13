package gstinit

import (
	"sync"

	"github.com/go-gst/go-gst/gst"
)

var initOnce sync.Once

func InitGST() {
	initOnce.Do(func() {
		gst.Init(nil)
	})
}
