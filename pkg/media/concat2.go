package media

import (
	"context"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
	"stream.place/streamplace/pkg/media/segchanman"
)

func NewConcatBin(ctx context.Context, segCh <-chan *segchanman.Seg) (*gst.Bin, error) {
	bin := gst.NewBin("concat")

	appSrc, err := gst.NewElementWithProperties("appsrc", map[string]interface{}{
		"name": "concat-appsrc",
	})
	if err != nil {
		return nil, err
	}
	err = bin.Add(appSrc)
	if err != nil {
		return nil, err
	}

	src := app.SrcFromElement(appSrc)
	go func() {
		select {
		case <-ctx.Done():
			return
		case seg := <-segCh:
			buffer := gst.NewBufferWithSize(int64(len(seg.Data)))
			buffer.Map(gst.MapWrite).WriteData(seg.Data)
			defer buffer.Unmap()
			src.PushBuffer(buffer)
			src.EndStream()
		}
	}()

	ghost := gst.NewGhostPad("src", appSrc.GetStaticPad("src"))
	bin.AddPad(ghost.Pad)

	return bin, nil
}
