package media

import (
	"context"
	"sync"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/base"
	"github.com/google/uuid"
	"stream.place/streamplace/pkg/log"
)

var MP4Syncs = map[string]*mp4sync{}
var MP4SyncsMutex sync.Mutex

func NewMP4Sync(sync *mp4sync) string {
	MP4SyncsMutex.Lock()
	defer MP4SyncsMutex.Unlock()
	uu, err := uuid.NewV7()
	if err != nil {
		panic("failed to generate UUID: " + err.Error())
	}
	MP4Syncs[uu.String()] = sync
	return uu.String()
}

func GetMP4Sync(id string) *mp4sync {
	MP4SyncsMutex.Lock()
	defer MP4SyncsMutex.Unlock()
	return MP4Syncs[id]
}

func RegisterMP4Sync() bool {
	return gst.RegisterElement(
		// no plugin:
		nil,
		// The name of the element
		"mp4sync",
		// The rank of the element
		gst.RankNone,
		// The GoElement implementation for the element
		&mp4sync{},
		// The base subclass this element extends
		base.ExtendsBaseTransform,
	)
}

type mp4sync struct {
	instanceID string
	buffers    []*gst.Buffer
}

// ClassInit is the place where you define pads and properties
func (*mp4sync) ClassInit(klass *glib.ObjectClass) {
	class := gst.ToElementClass(klass)
	class.SetMetadata(
		"custom base transform",
		"Transform/demo",
		"custom base transform",
		"Wilhelm Bartel <bartel.wilhelm@gmail.com>",
	)
	class.AddPadTemplate(gst.NewPadTemplate(
		"src",
		gst.PadDirectionSource,
		gst.PadPresenceAlways,
		gst.NewCapsFromString("video/x-h264,parsed=true"),
	))
	class.AddPadTemplate(gst.NewPadTemplate(
		"sink",
		gst.PadDirectionSink,
		gst.PadPresenceAlways,
		gst.NewCapsFromString("video/x-h264,parsed=true"),
	))
	var properties = []*glib.ParamSpec{
		glib.NewStringParam(
			"instance-id",           // The name of the parameter
			"Instance ID",           // The long name for the parameter
			"Instance ID",           // A blurb about the parameter
			nil,                     // A default value for the parameter
			glib.ParameterReadWrite, // Flags for the parameter
		),
	}
	class.InstallProperties(properties)
}

// SetProperty gets called for every property. The id is the index in the slice defined above.
func (s *mp4sync) SetProperty(self *glib.Object, id uint, value *glib.Value) {

}

// GetProperty is called to retrieve the value of the property at index `id` in the properties
// slice provided at ClassInit.
func (s *mp4sync) GetProperty(self *glib.Object, id uint) *glib.Value {
	if id == 0 {
		val, err := glib.GValue(s.instanceID)
		if err != nil {
			panic("failed to convert instance ID to GValue: " + err.Error())
		}
		return val
	}
	return nil
}

// New is called by the bindings to create a new instance of your go element. Use this to initialize channels, maps, etc.
//
// Think of New like the constructor of your struct
func (s *mp4sync) New() glib.GoObjectSubclass {
	instanceID := NewMP4Sync(s)
	return &mp4sync{instanceID: instanceID}
}

// InstanceInit should initialize the element. Keep in mind that the properties are not yet present. When this is called.
func (s *mp4sync) InstanceInit(instance *glib.Object) {}

func (s *mp4sync) Constructed(o *glib.Object) {}

func (s *mp4sync) Finalize(o *glib.Object) {}

// see base.GstBaseTransformImpl interface for the method signatures of the virtual methods
//
// it is not required to implement all methods
var _ base.GstBaseTransformImpl = nil

func (s *mp4sync) SubmitInputBuffer(self *base.GstBaseTransform, isDiscont bool, input *gst.Buffer) gst.FlowReturn {
	log.Warn(context.Background(), "got input buffer", "isDiscont", isDiscont)
	s.buffers = append(s.buffers, input)
	return gst.FlowOK
}

func (s *mp4sync) GenerateOutput(self *base.GstBaseTransform) (gst.FlowReturn, *gst.Buffer) {
	log.Warn(context.Background(), "generating output")
	if len(s.buffers) == 0 {
		log.Warn(context.Background(), "no buffers to generate output")
		return gst.FlowEOS, nil
	}
	buf := s.buffers[0]
	s.buffers = s.buffers[1:]
	log.Warn(context.Background(), "generated output", "buffer", buf)
	return gst.FlowOK, buf
}
