package iroh_streamplace

// #include <iroh_streamplace.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_iroh_streamplace_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_iroh_streamplace_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	FfiConverterDataHandlerINSTANCE.register()
	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_iroh_streamplace_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("iroh_streamplace: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_datahandler_handle_data()
		})
		if checksum != 27893 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_datahandler_handle_data: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_endpoint_node_addr()
		})
		if checksum != 17254 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_endpoint_node_addr: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_nodeaddr_direct_addresses()
		})
		if checksum != 17536 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_nodeaddr_direct_addresses: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_nodeaddr_equal()
		})
		if checksum != 15520 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_nodeaddr_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_nodeaddr_node_id()
		})
		if checksum != 35476 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_nodeaddr_node_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_nodeaddr_relay_url()
		})
		if checksum != 18967 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_nodeaddr_relay_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_publickey_equal()
		})
		if checksum != 25030 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_publickey_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_publickey_fmt_short()
		})
		if checksum != 57639 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_publickey_fmt_short: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_publickey_to_bytes()
		})
		if checksum != 8223 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_publickey_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_receiver_node_addr()
		})
		if checksum != 10730 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_receiver_node_addr: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_receiver_subscribe()
		})
		if checksum != 24145 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_receiver_subscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_receiver_unsubscribe()
		})
		if checksum != 21760 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_receiver_unsubscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_sender_node_addr()
		})
		if checksum != 38541 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_sender_node_addr: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_sender_send()
		})
		if checksum != 23930 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_sender_send: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_endpoint_new()
		})
		if checksum != 60672 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_endpoint_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_nodeaddr_new()
		})
		if checksum != 28044 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_nodeaddr_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_publickey_from_bytes()
		})
		if checksum != 54187 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_publickey_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_publickey_from_string()
		})
		if checksum != 62773 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_publickey_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_receiver_new()
		})
		if checksum != 35153 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_receiver_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_sender_new()
		})
		if checksum != 27594 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_sender_new: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

type FfiConverterBytes struct{}

var FfiConverterBytesINSTANCE = FfiConverterBytes{}

func (c FfiConverterBytes) Lower(value []byte) C.RustBuffer {
	return LowerIntoRustBuffer[[]byte](c, value)
}

func (c FfiConverterBytes) Write(writer io.Writer, value []byte) {
	if len(value) > math.MaxInt32 {
		panic("[]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := writer.Write(value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing []byte, expected %d, written %d", len(value), write_length))
	}
}

func (c FfiConverterBytes) Lift(rb RustBufferI) []byte {
	return LiftFromRustBuffer[[]byte](c, rb)
}

func (c FfiConverterBytes) Read(reader io.Reader) []byte {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct{}

func (FfiDestroyerBytes) Destroy(_ []byte) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type DataHandler interface {
	HandleData(topic string, data []byte)
}
type DataHandlerImpl struct {
	ffiObject FfiObject
}

func (_self *DataHandlerImpl) HandleData(topic string, data []byte) {
	_pointer := _self.ffiObject.incrementPointer("DataHandler")
	defer _self.ffiObject.decrementPointer()
	uniffiRustCallAsync[struct{}](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_datahandler_handle_data(
			_pointer, FfiConverterStringINSTANCE.Lower(topic), FfiConverterBytesINSTANCE.Lower(data)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

}
func (object *DataHandlerImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDataHandler struct {
	handleMap *concurrentHandleMap[DataHandler]
}

var FfiConverterDataHandlerINSTANCE = FfiConverterDataHandler{
	handleMap: newConcurrentHandleMap[DataHandler](),
}

func (c FfiConverterDataHandler) Lift(pointer unsafe.Pointer) DataHandler {
	result := &DataHandlerImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_datahandler(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_datahandler(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*DataHandlerImpl).Destroy)
	return result
}

func (c FfiConverterDataHandler) Read(reader io.Reader) DataHandler {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDataHandler) Lower(value DataHandler) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterDataHandler) Write(writer io.Writer, value DataHandler) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDataHandler struct{}

func (_ FfiDestroyerDataHandler) Destroy(value DataHandler) {
	if val, ok := value.(*DataHandlerImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *DataHandlerImpl")
	}
}

type uniffiCallbackResult C.int8_t

const (
	uniffiIdxCallbackFree               uniffiCallbackResult = 0
	uniffiCallbackResultSuccess         uniffiCallbackResult = 0
	uniffiCallbackResultError           uniffiCallbackResult = 1
	uniffiCallbackUnexpectedResultError uniffiCallbackResult = 2
	uniffiCallbackCancelled             uniffiCallbackResult = 3
)

type concurrentHandleMap[T any] struct {
	handles       map[uint64]T
	currentHandle uint64
	lock          sync.RWMutex
}

func newConcurrentHandleMap[T any]() *concurrentHandleMap[T] {
	return &concurrentHandleMap[T]{
		handles: map[uint64]T{},
	}
}

func (cm *concurrentHandleMap[T]) insert(obj T) uint64 {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.currentHandle = cm.currentHandle + 1
	cm.handles[cm.currentHandle] = obj
	return cm.currentHandle
}

func (cm *concurrentHandleMap[T]) remove(handle uint64) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	delete(cm.handles, handle)
}

func (cm *concurrentHandleMap[T]) tryGet(handle uint64) (T, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	val, ok := cm.handles[handle]
	return val, ok
}

//export iroh_streamplace_cgo_dispatchCallbackInterfaceDataHandlerMethod0
func iroh_streamplace_cgo_dispatchCallbackInterfaceDataHandlerMethod0(uniffiHandle C.uint64_t, topic C.RustBuffer, data C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterDataHandlerINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	result := make(chan C.UniffiForeignFutureStructVoid, 1)
	cancel := make(chan struct{}, 1)
	guardHandle := cgo.NewHandle(cancel)
	*uniffiOutReturn = C.UniffiForeignFuture{
		handle: C.uint64_t(guardHandle),
		free:   C.UniffiForeignFutureFree(C.iroh_streamplace_uniffiFreeGorutine),
	}

	// Wait for compleation or cancel
	go func() {
		select {
		case <-cancel:
		case res := <-result:
			C.call_UniffiForeignFutureCompleteVoid(uniffiFutureCallback, uniffiCallbackData, res)
		}
	}()

	// Eval callback asynchroniously
	go func() {
		asyncResult := &C.UniffiForeignFutureStructVoid{}
		defer func() {
			result <- *asyncResult
		}()

		uniffiObj.HandleData(
			FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: topic,
			}),
			FfiConverterBytesINSTANCE.Lift(GoRustBuffer{
				inner: data,
			}),
		)

	}()
}

var UniffiVTableCallbackInterfaceDataHandlerINSTANCE = C.UniffiVTableCallbackInterfaceDataHandler{
	handleData: (C.UniffiCallbackInterfaceDataHandlerMethod0)(C.iroh_streamplace_cgo_dispatchCallbackInterfaceDataHandlerMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.iroh_streamplace_cgo_dispatchCallbackInterfaceDataHandlerFree),
}

//export iroh_streamplace_cgo_dispatchCallbackInterfaceDataHandlerFree
func iroh_streamplace_cgo_dispatchCallbackInterfaceDataHandlerFree(handle C.uint64_t) {
	FfiConverterDataHandlerINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterDataHandler) register() {
	C.uniffi_iroh_streamplace_fn_init_callback_vtable_datahandler(&UniffiVTableCallbackInterfaceDataHandlerINSTANCE)
}

type EndpointInterface interface {
	NodeAddr() *NodeAddr
}
type Endpoint struct {
	ffiObject FfiObject
}

// Create a new endpoint.
func NewEndpoint() (*Endpoint, *Error) {
	res, err := uniffiRustCallAsync[Error](
		FfiConverterErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Endpoint {
			return FfiConverterEndpointINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_constructor_endpoint_new(),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	return res, err
}

func (_self *Endpoint) NodeAddr() *NodeAddr {
	_pointer := _self.ffiObject.incrementPointer("*Endpoint")
	defer _self.ffiObject.decrementPointer()
	res, _ := uniffiRustCallAsync[struct{}](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *NodeAddr {
			return FfiConverterNodeAddrINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_endpoint_node_addr(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	return res
}
func (object *Endpoint) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterEndpoint struct{}

var FfiConverterEndpointINSTANCE = FfiConverterEndpoint{}

func (c FfiConverterEndpoint) Lift(pointer unsafe.Pointer) *Endpoint {
	result := &Endpoint{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_endpoint(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_endpoint(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Endpoint).Destroy)
	return result
}

func (c FfiConverterEndpoint) Read(reader io.Reader) *Endpoint {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterEndpoint) Lower(value *Endpoint) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Endpoint")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterEndpoint) Write(writer io.Writer, value *Endpoint) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerEndpoint struct{}

func (_ FfiDestroyerEndpoint) Destroy(value *Endpoint) {
	value.Destroy()
}

// A peer and it's addressing information.
type NodeAddrInterface interface {
	// Get the direct addresses of this peer.
	DirectAddresses() []string
	// Returns true if both NodeAddr's have the same values
	Equal(other *NodeAddr) bool
	NodeId() *PublicKey
	// Get the home relay URL for this peer
	RelayUrl() *string
}

// A peer and it's addressing information.
type NodeAddr struct {
	ffiObject FfiObject
}

// Create a new [`NodeAddr`] with empty [`AddrInfo`].
func NewNodeAddr(nodeId *PublicKey, derpUrl *string, addresses []string) *NodeAddr {
	return FfiConverterNodeAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_constructor_nodeaddr_new(FfiConverterPublicKeyINSTANCE.Lower(nodeId), FfiConverterOptionalStringINSTANCE.Lower(derpUrl), FfiConverterSequenceStringINSTANCE.Lower(addresses), _uniffiStatus)
	}))
}

// Get the direct addresses of this peer.
func (_self *NodeAddr) DirectAddresses() []string {
	_pointer := _self.ffiObject.incrementPointer("*NodeAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_nodeaddr_direct_addresses(
				_pointer, _uniffiStatus),
		}
	}))
}

// Returns true if both NodeAddr's have the same values
func (_self *NodeAddr) Equal(other *NodeAddr) bool {
	_pointer := _self.ffiObject.incrementPointer("*NodeAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_streamplace_fn_method_nodeaddr_equal(
			_pointer, FfiConverterNodeAddrINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *NodeAddr) NodeId() *PublicKey {
	_pointer := _self.ffiObject.incrementPointer("*NodeAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterPublicKeyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_nodeaddr_node_id(
			_pointer, _uniffiStatus)
	}))
}

// Get the home relay URL for this peer
func (_self *NodeAddr) RelayUrl() *string {
	_pointer := _self.ffiObject.incrementPointer("*NodeAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_nodeaddr_relay_url(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *NodeAddr) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterNodeAddr struct{}

var FfiConverterNodeAddrINSTANCE = FfiConverterNodeAddr{}

func (c FfiConverterNodeAddr) Lift(pointer unsafe.Pointer) *NodeAddr {
	result := &NodeAddr{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_nodeaddr(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_nodeaddr(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*NodeAddr).Destroy)
	return result
}

func (c FfiConverterNodeAddr) Read(reader io.Reader) *NodeAddr {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterNodeAddr) Lower(value *NodeAddr) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*NodeAddr")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterNodeAddr) Write(writer io.Writer, value *NodeAddr) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerNodeAddr struct{}

func (_ FfiDestroyerNodeAddr) Destroy(value *NodeAddr) {
	value.Destroy()
}

// A public key.
//
// The key itself is just a 32 byte array, but a key has associated crypto
// information that is cached for performance reasons.
type PublicKeyInterface interface {
	// Returns true if the PublicKeys are equal
	Equal(other *PublicKey) bool
	// Convert to a base32 string limited to the first 10 bytes for a friendly string
	// representation of the key.
	FmtShort() string
	// Express the PublicKey as a byte array
	ToBytes() []byte
}

// A public key.
//
// The key itself is just a 32 byte array, but a key has associated crypto
// information that is cached for performance reasons.
type PublicKey struct {
	ffiObject FfiObject
}

// Make a PublicKey from byte array
func PublicKeyFromBytes(bytes []byte) (*PublicKey, *Error) {
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_constructor_publickey_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PublicKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPublicKeyINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

// Make a PublicKey from base32 string
func PublicKeyFromString(s string) (*PublicKey, *Error) {
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_constructor_publickey_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PublicKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPublicKeyINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

// Returns true if the PublicKeys are equal
func (_self *PublicKey) Equal(other *PublicKey) bool {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_streamplace_fn_method_publickey_equal(
			_pointer, FfiConverterPublicKeyINSTANCE.Lower(other), _uniffiStatus)
	}))
}

// Convert to a base32 string limited to the first 10 bytes for a friendly string
// representation of the key.
func (_self *PublicKey) FmtShort() string {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_publickey_fmt_short(
				_pointer, _uniffiStatus),
		}
	}))
}

// Express the PublicKey as a byte array
func (_self *PublicKey) ToBytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_publickey_to_bytes(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *PublicKey) String() string {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_publickey_uniffi_trait_display(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *PublicKey) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPublicKey struct{}

var FfiConverterPublicKeyINSTANCE = FfiConverterPublicKey{}

func (c FfiConverterPublicKey) Lift(pointer unsafe.Pointer) *PublicKey {
	result := &PublicKey{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_publickey(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_publickey(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*PublicKey).Destroy)
	return result
}

func (c FfiConverterPublicKey) Read(reader io.Reader) *PublicKey {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPublicKey) Lower(value *PublicKey) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PublicKey")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterPublicKey) Write(writer io.Writer, value *PublicKey) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPublicKey struct{}

func (_ FfiDestroyerPublicKey) Destroy(value *PublicKey) {
	value.Destroy()
}

type ReceiverInterface interface {
	NodeAddr() *NodeAddr
	// Subscribe to the given topic on the remote.
	Subscribe(remoteId *PublicKey, topic string) *Error
	// Unsubscribe from this topic on the remote.
	Unsubscribe(remoteId *PublicKey, topic string) *Error
}
type Receiver struct {
	ffiObject FfiObject
}

// Create a new receiver.
func NewReceiver(endpoint *Endpoint, handler DataHandler) (*Receiver, *Error) {
	res, err := uniffiRustCallAsync[Error](
		FfiConverterErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Receiver {
			return FfiConverterReceiverINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_constructor_receiver_new(FfiConverterEndpointINSTANCE.Lower(endpoint), FfiConverterDataHandlerINSTANCE.Lower(handler)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	return res, err
}

func (_self *Receiver) NodeAddr() *NodeAddr {
	_pointer := _self.ffiObject.incrementPointer("*Receiver")
	defer _self.ffiObject.decrementPointer()
	res, _ := uniffiRustCallAsync[struct{}](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *NodeAddr {
			return FfiConverterNodeAddrINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_receiver_node_addr(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	return res
}

// Subscribe to the given topic on the remote.
func (_self *Receiver) Subscribe(remoteId *PublicKey, topic string) *Error {
	_pointer := _self.ffiObject.incrementPointer("*Receiver")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[Error](
		FfiConverterErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_receiver_subscribe(
			_pointer, FfiConverterPublicKeyINSTANCE.Lower(remoteId), FfiConverterStringINSTANCE.Lower(topic)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	return err
}

// Unsubscribe from this topic on the remote.
func (_self *Receiver) Unsubscribe(remoteId *PublicKey, topic string) *Error {
	_pointer := _self.ffiObject.incrementPointer("*Receiver")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[Error](
		FfiConverterErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_receiver_unsubscribe(
			_pointer, FfiConverterPublicKeyINSTANCE.Lower(remoteId), FfiConverterStringINSTANCE.Lower(topic)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	return err
}
func (object *Receiver) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterReceiver struct{}

var FfiConverterReceiverINSTANCE = FfiConverterReceiver{}

func (c FfiConverterReceiver) Lift(pointer unsafe.Pointer) *Receiver {
	result := &Receiver{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_receiver(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_receiver(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Receiver).Destroy)
	return result
}

func (c FfiConverterReceiver) Read(reader io.Reader) *Receiver {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterReceiver) Lower(value *Receiver) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Receiver")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterReceiver) Write(writer io.Writer, value *Receiver) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerReceiver struct{}

func (_ FfiDestroyerReceiver) Destroy(value *Receiver) {
	value.Destroy()
}

type SenderInterface interface {
	NodeAddr() *NodeAddr
	// Sends the given data to all subscribers that have subscribed to this `key`.
	Send(key string, data []byte) *Error
}
type Sender struct {
	ffiObject FfiObject
}

// Create a new sender.
func NewSender(endpoint *Endpoint) (*Sender, *Error) {
	res, err := uniffiRustCallAsync[Error](
		FfiConverterErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Sender {
			return FfiConverterSenderINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_constructor_sender_new(FfiConverterEndpointINSTANCE.Lower(endpoint)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	return res, err
}

func (_self *Sender) NodeAddr() *NodeAddr {
	_pointer := _self.ffiObject.incrementPointer("*Sender")
	defer _self.ffiObject.decrementPointer()
	res, _ := uniffiRustCallAsync[struct{}](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *NodeAddr {
			return FfiConverterNodeAddrINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_sender_node_addr(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	return res
}

// Sends the given data to all subscribers that have subscribed to this `key`.
func (_self *Sender) Send(key string, data []byte) *Error {
	_pointer := _self.ffiObject.incrementPointer("*Sender")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[Error](
		FfiConverterErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_sender_send(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterBytesINSTANCE.Lower(data)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	return err
}
func (object *Sender) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSender struct{}

var FfiConverterSenderINSTANCE = FfiConverterSender{}

func (c FfiConverterSender) Lift(pointer unsafe.Pointer) *Sender {
	result := &Sender{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_sender(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_sender(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Sender).Destroy)
	return result
}

func (c FfiConverterSender) Read(reader io.Reader) *Sender {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSender) Lower(value *Sender) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Sender")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSender) Write(writer io.Writer, value *Sender) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSender struct{}

func (_ FfiDestroyerSender) Destroy(value *Sender) {
	value.Destroy()
}

// An Error.
type Error struct {
	err error
}

// Convience method to turn *Error into error
// Avoiding treating nil pointer as non nil error interface
func (err *Error) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err Error) Error() string {
	return fmt.Sprintf("Error: %s", err.err.Error())
}

func (err Error) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrErrorIrohBind = fmt.Errorf("ErrorIrohBind")
var ErrErrorInvalidUrl = fmt.Errorf("ErrorInvalidUrl")
var ErrErrorIrohConnect = fmt.Errorf("ErrorIrohConnect")
var ErrErrorInvalidNetworkAddress = fmt.Errorf("ErrorInvalidNetworkAddress")
var ErrErrorMissingConnection = fmt.Errorf("ErrorMissingConnection")
var ErrErrorInvalidPublicKey = fmt.Errorf("ErrorInvalidPublicKey")
var ErrErrorIrpc = fmt.Errorf("ErrorIrpc")

// Variant structs
type ErrorIrohBind struct {
	message string
}

func NewErrorIrohBind() *Error {
	return &Error{err: &ErrorIrohBind{}}
}

func (e ErrorIrohBind) destroy() {
}

func (err ErrorIrohBind) Error() string {
	return fmt.Sprintf("IrohBind: %s", err.message)
}

func (self ErrorIrohBind) Is(target error) bool {
	return target == ErrErrorIrohBind
}

type ErrorInvalidUrl struct {
	message string
}

func NewErrorInvalidUrl() *Error {
	return &Error{err: &ErrorInvalidUrl{}}
}

func (e ErrorInvalidUrl) destroy() {
}

func (err ErrorInvalidUrl) Error() string {
	return fmt.Sprintf("InvalidUrl: %s", err.message)
}

func (self ErrorInvalidUrl) Is(target error) bool {
	return target == ErrErrorInvalidUrl
}

type ErrorIrohConnect struct {
	message string
}

func NewErrorIrohConnect() *Error {
	return &Error{err: &ErrorIrohConnect{}}
}

func (e ErrorIrohConnect) destroy() {
}

func (err ErrorIrohConnect) Error() string {
	return fmt.Sprintf("IrohConnect: %s", err.message)
}

func (self ErrorIrohConnect) Is(target error) bool {
	return target == ErrErrorIrohConnect
}

type ErrorInvalidNetworkAddress struct {
	message string
}

func NewErrorInvalidNetworkAddress() *Error {
	return &Error{err: &ErrorInvalidNetworkAddress{}}
}

func (e ErrorInvalidNetworkAddress) destroy() {
}

func (err ErrorInvalidNetworkAddress) Error() string {
	return fmt.Sprintf("InvalidNetworkAddress: %s", err.message)
}

func (self ErrorInvalidNetworkAddress) Is(target error) bool {
	return target == ErrErrorInvalidNetworkAddress
}

type ErrorMissingConnection struct {
	message string
}

func NewErrorMissingConnection() *Error {
	return &Error{err: &ErrorMissingConnection{}}
}

func (e ErrorMissingConnection) destroy() {
}

func (err ErrorMissingConnection) Error() string {
	return fmt.Sprintf("MissingConnection: %s", err.message)
}

func (self ErrorMissingConnection) Is(target error) bool {
	return target == ErrErrorMissingConnection
}

type ErrorInvalidPublicKey struct {
	message string
}

func NewErrorInvalidPublicKey() *Error {
	return &Error{err: &ErrorInvalidPublicKey{}}
}

func (e ErrorInvalidPublicKey) destroy() {
}

func (err ErrorInvalidPublicKey) Error() string {
	return fmt.Sprintf("InvalidPublicKey: %s", err.message)
}

func (self ErrorInvalidPublicKey) Is(target error) bool {
	return target == ErrErrorInvalidPublicKey
}

type ErrorIrpc struct {
	message string
}

func NewErrorIrpc() *Error {
	return &Error{err: &ErrorIrpc{}}
}

func (e ErrorIrpc) destroy() {
}

func (err ErrorIrpc) Error() string {
	return fmt.Sprintf("Irpc: %s", err.message)
}

func (self ErrorIrpc) Is(target error) bool {
	return target == ErrErrorIrpc
}

type FfiConverterError struct{}

var FfiConverterErrorINSTANCE = FfiConverterError{}

func (c FfiConverterError) Lift(eb RustBufferI) *Error {
	return LiftFromRustBuffer[*Error](c, eb)
}

func (c FfiConverterError) Lower(value *Error) C.RustBuffer {
	return LowerIntoRustBuffer[*Error](c, value)
}

func (c FfiConverterError) Read(reader io.Reader) *Error {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &Error{&ErrorIrohBind{message}}
	case 2:
		return &Error{&ErrorInvalidUrl{message}}
	case 3:
		return &Error{&ErrorIrohConnect{message}}
	case 4:
		return &Error{&ErrorInvalidNetworkAddress{message}}
	case 5:
		return &Error{&ErrorMissingConnection{message}}
	case 6:
		return &Error{&ErrorInvalidPublicKey{message}}
	case 7:
		return &Error{&ErrorIrpc{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterError.Read()", errorID))
	}

}

func (c FfiConverterError) Write(writer io.Writer, value *Error) {
	switch variantValue := value.err.(type) {
	case *ErrorIrohBind:
		writeInt32(writer, 1)
	case *ErrorInvalidUrl:
		writeInt32(writer, 2)
	case *ErrorIrohConnect:
		writeInt32(writer, 3)
	case *ErrorInvalidNetworkAddress:
		writeInt32(writer, 4)
	case *ErrorMissingConnection:
		writeInt32(writer, 5)
	case *ErrorInvalidPublicKey:
		writeInt32(writer, 6)
	case *ErrorIrpc:
		writeInt32(writer, 7)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterError.Write", value))
	}
}

type FfiDestroyerError struct{}

func (_ FfiDestroyerError) Destroy(value *Error) {
	switch variantValue := value.err.(type) {
	case ErrorIrohBind:
		variantValue.destroy()
	case ErrorInvalidUrl:
		variantValue.destroy()
	case ErrorIrohConnect:
		variantValue.destroy()
	case ErrorInvalidNetworkAddress:
		variantValue.destroy()
	case ErrorMissingConnection:
		variantValue.destroy()
	case ErrorInvalidPublicKey:
		variantValue.destroy()
	case ErrorIrpc:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerError.Destroy", value))
	}
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) C.RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterSequenceString struct{}

var FfiConverterSequenceStringINSTANCE = FfiConverterSequenceString{}

func (c FfiConverterSequenceString) Lift(rb RustBufferI) []string {
	return LiftFromRustBuffer[[]string](c, rb)
}

func (c FfiConverterSequenceString) Read(reader io.Reader) []string {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]string, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterStringINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceString) Lower(value []string) C.RustBuffer {
	return LowerIntoRustBuffer[[]string](c, value)
}

func (c FfiConverterSequenceString) Write(writer io.Writer, value []string) {
	if len(value) > math.MaxInt32 {
		panic("[]string is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterStringINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceString struct{}

func (FfiDestroyerSequenceString) Destroy(sequence []string) {
	for _, value := range sequence {
		FfiDestroyerString{}.Destroy(value)
	}
}

const (
	uniffiRustFuturePollReady      int8 = 0
	uniffiRustFuturePollMaybeReady int8 = 1
)

type rustFuturePollFunc func(C.uint64_t, C.UniffiRustFutureContinuationCallback, C.uint64_t)
type rustFutureCompleteFunc[T any] func(C.uint64_t, *C.RustCallStatus) T
type rustFutureFreeFunc func(C.uint64_t)

//export iroh_streamplace_uniffiFutureContinuationCallback
func iroh_streamplace_uniffiFutureContinuationCallback(data C.uint64_t, pollResult C.int8_t) {
	h := cgo.Handle(uintptr(data))
	waiter := h.Value().(chan int8)
	waiter <- int8(pollResult)
}

func uniffiRustCallAsync[E any, T any, F any](
	errConverter BufReader[*E],
	completeFunc rustFutureCompleteFunc[F],
	liftFunc func(F) T,
	rustFuture C.uint64_t,
	pollFunc rustFuturePollFunc,
	freeFunc rustFutureFreeFunc,
) (T, *E) {
	defer freeFunc(rustFuture)

	pollResult := int8(-1)
	waiter := make(chan int8, 1)

	chanHandle := cgo.NewHandle(waiter)
	defer chanHandle.Delete()

	for pollResult != uniffiRustFuturePollReady {
		pollFunc(
			rustFuture,
			(C.UniffiRustFutureContinuationCallback)(C.iroh_streamplace_uniffiFutureContinuationCallback),
			C.uint64_t(chanHandle),
		)
		pollResult = <-waiter
	}

	var goValue T
	var ffiValue F
	var err *E

	ffiValue, err = rustCallWithError(errConverter, func(status *C.RustCallStatus) F {
		return completeFunc(rustFuture, status)
	})
	if err != nil {
		return goValue, err
	}
	return liftFunc(ffiValue), nil
}

//export iroh_streamplace_uniffiFreeGorutine
func iroh_streamplace_uniffiFreeGorutine(data C.uint64_t) {
	handle := cgo.Handle(uintptr(data))
	defer handle.Delete()

	guard := handle.Value().(chan struct{})
	guard <- struct{}{}
}
