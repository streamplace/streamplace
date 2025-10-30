package iroh_streamplace

// #include <iroh_streamplace.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"runtime"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"time"
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
	FfiConverterGoSignerINSTANCE.register()
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
			return C.uniffi_iroh_streamplace_checksum_func_get_manifest_and_cert()
		})
		if checksum != 17550 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_get_manifest_and_cert: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_get_manifests()
		})
		if checksum != 17 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_get_manifests: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_init_logging()
		})
		if checksum != 40911 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_init_logging: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_init_logging_with_level()
		})
		if checksum != 49532 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_init_logging_with_level: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_node_id_from_ticket()
		})
		if checksum != 36085 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_node_id_from_ticket: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_resign()
		})
		if checksum != 32678 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_resign: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_sign()
		})
		if checksum != 23786 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_sign: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_sign_with_ingredients()
		})
		if checksum != 63680 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_sign_with_ingredients: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_func_subscribe_item_debug()
		})
		if checksum != 8233 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_subscribe_item_debug: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_datahandler_handle_data()
		})
		if checksum != 48772 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_datahandler_handle_data: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_db_iter_with_opts()
		})
		if checksum != 3815 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_db_iter_with_opts: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_db_shutdown()
		})
		if checksum != 9825 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_db_shutdown: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_db_subscribe()
		})
		if checksum != 415 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_db_subscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_db_subscribe_with_opts()
		})
		if checksum != 17612 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_db_subscribe_with_opts: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_db_write()
		})
		if checksum != 5247 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_db_write: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_filter_global()
		})
		if checksum != 47323 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_filter_global: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_filter_scope()
		})
		if checksum != 23022 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_filter_scope: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_filter_scopes()
		})
		if checksum != 39037 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_filter_scopes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_filter_stream()
		})
		if checksum != 37766 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_filter_stream: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_filter_time_from()
		})
		if checksum != 62275 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_filter_time_from: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_filter_time_range()
		})
		if checksum != 2045 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_filter_time_range: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_filter_timestamps()
		})
		if checksum != 27584 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_filter_timestamps: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_gosigner_sign()
		})
		if checksum != 50597 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_gosigner_sign: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_add_tickets()
		})
		if checksum != 8701 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_add_tickets: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_db()
		})
		if checksum != 39096 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_db: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_join_peers()
		})
		if checksum != 56647 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_join_peers: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_node_id()
		})
		if checksum != 55009 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_node_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_node_scope()
		})
		if checksum != 16912 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_node_scope: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_send_segment()
		})
		if checksum != 18989 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_send_segment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_shutdown()
		})
		if checksum != 18129 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_shutdown: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_subscribe()
		})
		if checksum != 17951 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_subscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_ticket()
		})
		if checksum != 37020 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_ticket: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_node_unsubscribe()
		})
		if checksum != 35396 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_node_unsubscribe: UniFFI API checksum mismatch")
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
			return C.uniffi_iroh_streamplace_checksum_method_publickey_as_vec()
		})
		if checksum != 32346 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_publickey_as_vec: UniFFI API checksum mismatch")
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
			return C.uniffi_iroh_streamplace_checksum_method_subscriberesponse_next_raw()
		})
		if checksum != 55650 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_subscriberesponse_next_raw: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_writescope_put()
		})
		if checksum != 50543 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_writescope_put: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_filter_new()
		})
		if checksum != 5241 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_filter_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_node_forwarder()
		})
		if checksum != 7334 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_node_forwarder: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_node_receiver()
		})
		if checksum != 33844 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_node_receiver: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_node_sender()
		})
		if checksum != 44785 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_node_sender: UniFFI API checksum mismatch")
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
		if checksum != 57602 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_publickey_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_publickey_from_string()
		})
		if checksum != 45922 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_publickey_from_string: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

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
	if err != nil && err != io.EOF {
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
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct{}

func (FfiDestroyerBytes) Destroy(_ []byte) {}

// FfiConverterDuration converts between uniffi duration and Go duration.
type FfiConverterDuration struct{}

var FfiConverterDurationINSTANCE = FfiConverterDuration{}

func (c FfiConverterDuration) Lift(rb RustBufferI) time.Duration {
	return LiftFromRustBuffer[time.Duration](c, rb)
}

func (c FfiConverterDuration) Read(reader io.Reader) time.Duration {
	sec := readUint64(reader)
	nsec := readUint32(reader)
	return time.Duration(sec*1_000_000_000 + uint64(nsec))
}

func (c FfiConverterDuration) Lower(value time.Duration) C.RustBuffer {
	return LowerIntoRustBuffer[time.Duration](c, value)
}

func (c FfiConverterDuration) Write(writer io.Writer, value time.Duration) {
	if value.Nanoseconds() < 0 {
		// Rust does not support negative durations:
		// https://www.reddit.com/r/rust/comments/ljl55u/why_rusts_duration_not_supporting_negative_values/
		// This panic is very bad, because it depends on user input, and in Go user input related
		// error are supposed to be returned as errors, and not cause panics. However, with the
		// current architecture, its not possible to return an error from here, so panic is used as
		// the only other option to signal an error.
		panic("negative duration is not allowed")
	}

	writeUint64(writer, uint64(value)/1_000_000_000)
	writeUint32(writer, uint32(uint64(value)%1_000_000_000))
}

type FfiDestroyerDuration struct{}

func (FfiDestroyerDuration) Destroy(_ time.Duration) {}

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

// DataHandler trait that is exported to go for receiving data callbacks.
type DataHandler interface {
	HandleData(from *PublicKey, topic string, data []byte)
}

// DataHandler trait that is exported to go for receiving data callbacks.
type DataHandlerImpl struct {
	ffiObject FfiObject
}

func (_self *DataHandlerImpl) HandleData(from *PublicKey, topic string, data []byte) {
	_pointer := _self.ffiObject.incrementPointer("DataHandler")
	defer _self.ffiObject.decrementPointer()
	uniffiRustCallAsync[error](
		nil,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_datahandler_handle_data(
			_pointer, FfiConverterPublicKeyINSTANCE.Lower(from), FfiConverterStringINSTANCE.Lower(topic), FfiConverterBytesINSTANCE.Lower(data)),
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
func iroh_streamplace_cgo_dispatchCallbackInterfaceDataHandlerMethod0(uniffiHandle C.uint64_t, from unsafe.Pointer, topic C.RustBuffer, data C.RustBuffer, uniffiFutureCallback C.UniffiForeignFutureCompleteVoid, uniffiCallbackData C.uint64_t, uniffiOutReturn *C.UniffiForeignFuture) {
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
			FfiConverterPublicKeyINSTANCE.Lift(from),
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

// Iroh-streamplace specific metadata database.
type DbInterface interface {
	IterWithOpts(filter *Filter) ([]Entry, error)
	// Shutdown the database client and all subscriptions.
	Shutdown() error
	Subscribe(filter *Filter) *SubscribeResponse
	// Subscribe with options.
	SubscribeWithOpts(opts SubscribeOpts) *SubscribeResponse
	Write(secret []byte) (*WriteScope, error)
}

// Iroh-streamplace specific metadata database.
type Db struct {
	ffiObject FfiObject
}

func (_self *Db) IterWithOpts(filter *Filter) ([]Entry, error) {
	_pointer := _self.ffiObject.incrementPointer("*Db")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SubscribeNextError](
		FfiConverterSubscribeNextErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_iroh_streamplace_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) []Entry {
			return FfiConverterSequenceEntryINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_db_iter_with_opts(
			_pointer, FfiConverterFilterINSTANCE.Lower(filter)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_rust_buffer(handle)
		},
	)

	if err == nil {
		return res, nil
	}

	return res, err
}

// Shutdown the database client and all subscriptions.
func (_self *Db) Shutdown() error {
	_pointer := _self.ffiObject.incrementPointer("*Db")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[ShutdownError](
		FfiConverterShutdownErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_db_shutdown(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	if err == nil {
		return nil
	}

	return err
}

func (_self *Db) Subscribe(filter *Filter) *SubscribeResponse {
	_pointer := _self.ffiObject.incrementPointer("*Db")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscribeResponseINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_db_subscribe(
			_pointer, FfiConverterFilterINSTANCE.Lower(filter), _uniffiStatus)
	}))
}

// Subscribe with options.
func (_self *Db) SubscribeWithOpts(opts SubscribeOpts) *SubscribeResponse {
	_pointer := _self.ffiObject.incrementPointer("*Db")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSubscribeResponseINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_db_subscribe_with_opts(
			_pointer, FfiConverterSubscribeOptsINSTANCE.Lower(opts), _uniffiStatus)
	}))
}

func (_self *Db) Write(secret []byte) (*WriteScope, error) {
	_pointer := _self.ffiObject.incrementPointer("*Db")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[WriteError](FfiConverterWriteError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_db_write(
			_pointer, FfiConverterBytesINSTANCE.Lower(secret), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *WriteScope
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWriteScopeINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Db) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDb struct{}

var FfiConverterDbINSTANCE = FfiConverterDb{}

func (c FfiConverterDb) Lift(pointer unsafe.Pointer) *Db {
	result := &Db{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_db(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_db(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Db).Destroy)
	return result
}

func (c FfiConverterDb) Read(reader io.Reader) *Db {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDb) Lower(value *Db) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Db")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterDb) Write(writer io.Writer, value *Db) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDb struct{}

func (_ FfiDestroyerDb) Destroy(value *Db) {
	value.Destroy()
}

// A filter for subscriptions and iteration.
type FilterInterface interface {
	// Restrict to the global namespace, no per stream data.
	Global() *Filter
	// Restrict to a single scope.
	Scope(scope *PublicKey) *Filter
	// Restrict to a set of scopes.
	Scopes(scopes []*PublicKey) *Filter
	// Restrict to one specific stream, no global data.
	Stream(stream []byte) *Filter
	// Restrict to a time range starting at min, unbounded at the top.
	TimeFrom(min uint64) *Filter
	// Restrict to a time range given in nanoseconds since unix epoch.
	TimeRange(min uint64, max uint64) *Filter
	// Restrict to a time range.
	Timestamps(min TimeBound, max TimeBound) *Filter
}

// A filter for subscriptions and iteration.
type Filter struct {
	ffiObject FfiObject
}

// Creates a new filter that matches everything.
func NewFilter() *Filter {
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_constructor_filter_new(_uniffiStatus)
	}))
}

// Restrict to the global namespace, no per stream data.
func (_self *Filter) Global() *Filter {
	_pointer := _self.ffiObject.incrementPointer("*Filter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_filter_global(
			_pointer, _uniffiStatus)
	}))
}

// Restrict to a single scope.
func (_self *Filter) Scope(scope *PublicKey) *Filter {
	_pointer := _self.ffiObject.incrementPointer("*Filter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_filter_scope(
			_pointer, FfiConverterPublicKeyINSTANCE.Lower(scope), _uniffiStatus)
	}))
}

// Restrict to a set of scopes.
func (_self *Filter) Scopes(scopes []*PublicKey) *Filter {
	_pointer := _self.ffiObject.incrementPointer("*Filter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_filter_scopes(
			_pointer, FfiConverterSequencePublicKeyINSTANCE.Lower(scopes), _uniffiStatus)
	}))
}

// Restrict to one specific stream, no global data.
func (_self *Filter) Stream(stream []byte) *Filter {
	_pointer := _self.ffiObject.incrementPointer("*Filter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_filter_stream(
			_pointer, FfiConverterBytesINSTANCE.Lower(stream), _uniffiStatus)
	}))
}

// Restrict to a time range starting at min, unbounded at the top.
func (_self *Filter) TimeFrom(min uint64) *Filter {
	_pointer := _self.ffiObject.incrementPointer("*Filter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_filter_time_from(
			_pointer, FfiConverterUint64INSTANCE.Lower(min), _uniffiStatus)
	}))
}

// Restrict to a time range given in nanoseconds since unix epoch.
func (_self *Filter) TimeRange(min uint64, max uint64) *Filter {
	_pointer := _self.ffiObject.incrementPointer("*Filter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_filter_time_range(
			_pointer, FfiConverterUint64INSTANCE.Lower(min), FfiConverterUint64INSTANCE.Lower(max), _uniffiStatus)
	}))
}

// Restrict to a time range.
func (_self *Filter) Timestamps(min TimeBound, max TimeBound) *Filter {
	_pointer := _self.ffiObject.incrementPointer("*Filter")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterFilterINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_filter_timestamps(
			_pointer, FfiConverterTimeBoundINSTANCE.Lower(min), FfiConverterTimeBoundINSTANCE.Lower(max), _uniffiStatus)
	}))
}
func (object *Filter) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterFilter struct{}

var FfiConverterFilterINSTANCE = FfiConverterFilter{}

func (c FfiConverterFilter) Lift(pointer unsafe.Pointer) *Filter {
	result := &Filter{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_filter(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_filter(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Filter).Destroy)
	return result
}

func (c FfiConverterFilter) Read(reader io.Reader) *Filter {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterFilter) Lower(value *Filter) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Filter")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterFilter) Write(writer io.Writer, value *Filter) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerFilter struct{}

func (_ FfiDestroyerFilter) Destroy(value *Filter) {
	value.Destroy()
}

type GoSigner interface {
	Sign(data []byte) ([]byte, error)
}
type GoSignerImpl struct {
	ffiObject FfiObject
}

func (_self *GoSignerImpl) Sign(data []byte) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("GoSigner")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[SpError](FfiConverterSpError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_gosigner_sign(
				_pointer, FfiConverterBytesINSTANCE.Lower(data), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *GoSignerImpl) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterGoSigner struct {
	handleMap *concurrentHandleMap[GoSigner]
}

var FfiConverterGoSignerINSTANCE = FfiConverterGoSigner{
	handleMap: newConcurrentHandleMap[GoSigner](),
}

func (c FfiConverterGoSigner) Lift(pointer unsafe.Pointer) GoSigner {
	result := &GoSignerImpl{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_gosigner(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_gosigner(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*GoSignerImpl).Destroy)
	return result
}

func (c FfiConverterGoSigner) Read(reader io.Reader) GoSigner {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterGoSigner) Lower(value GoSigner) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := unsafe.Pointer(uintptr(c.handleMap.insert(value)))
	return pointer

}

func (c FfiConverterGoSigner) Write(writer io.Writer, value GoSigner) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerGoSigner struct{}

func (_ FfiDestroyerGoSigner) Destroy(value GoSigner) {
	if val, ok := value.(*GoSignerImpl); ok {
		val.Destroy()
	} else {
		panic("Expected *GoSignerImpl")
	}
}

//export iroh_streamplace_cgo_dispatchCallbackInterfaceGoSignerMethod0
func iroh_streamplace_cgo_dispatchCallbackInterfaceGoSignerMethod0(uniffiHandle C.uint64_t, data C.RustBuffer, uniffiOutReturn *C.RustBuffer, callStatus *C.RustCallStatus) {
	handle := uint64(uniffiHandle)
	uniffiObj, ok := FfiConverterGoSignerINSTANCE.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}

	res, err :=
		uniffiObj.Sign(
			FfiConverterBytesINSTANCE.Lift(GoRustBuffer{
				inner: data,
			}),
		)

	if err != nil {
		var actualError *SpError
		if errors.As(err, &actualError) {
			*callStatus = C.RustCallStatus{
				code:     C.int8_t(uniffiCallbackResultError),
				errorBuf: FfiConverterSpErrorINSTANCE.Lower(actualError),
			}
		} else {
			*callStatus = C.RustCallStatus{
				code: C.int8_t(uniffiCallbackUnexpectedResultError),
			}
		}
		return
	}

	*uniffiOutReturn = FfiConverterBytesINSTANCE.Lower(res)
}

var UniffiVTableCallbackInterfaceGoSignerINSTANCE = C.UniffiVTableCallbackInterfaceGoSigner{
	sign: (C.UniffiCallbackInterfaceGoSignerMethod0)(C.iroh_streamplace_cgo_dispatchCallbackInterfaceGoSignerMethod0),

	uniffiFree: (C.UniffiCallbackInterfaceFree)(C.iroh_streamplace_cgo_dispatchCallbackInterfaceGoSignerFree),
}

//export iroh_streamplace_cgo_dispatchCallbackInterfaceGoSignerFree
func iroh_streamplace_cgo_dispatchCallbackInterfaceGoSignerFree(handle C.uint64_t) {
	FfiConverterGoSignerINSTANCE.handleMap.remove(uint64(handle))
}

func (c FfiConverterGoSigner) register() {
	C.uniffi_iroh_streamplace_fn_init_callback_vtable_gosigner(&UniffiVTableCallbackInterfaceGoSignerINSTANCE)
}

// Iroh-streamplace node that can send, forward or receive stream segments.
type NodeInterface interface {
	// Add tickets for remote peers
	AddTickets(peers []string) error
	// Get a handle to the db to watch for changes locally or globally.
	Db() *Db
	// Join peers by their node tickets.
	JoinPeers(peers []string) error
	// Get this node's node ID.
	NodeId() (*PublicKey, error)
	// Get a handle to the write scope for this node.
	//
	// This is equivalent to calling `db.write(...)` with the secret key used to create the node.
	NodeScope() *WriteScope
	// Send a segment to all subscribers of the given stream.
	SendSegment(key string, data []byte) error
	// Shutdown the node, including the streaming system and the metadata db.
	Shutdown() error
	// Subscribe to updates for a given stream from a remote node.
	Subscribe(key string, remoteId *PublicKey) error
	// Get this node's ticket.
	Ticket() (string, error)
	// Unsubscribe from updates for a given stream from a remote node.
	Unsubscribe(key string, remoteId *PublicKey) error
}

// Iroh-streamplace node that can send, forward or receive stream segments.
type Node struct {
	ffiObject FfiObject
}

func NodeForwarder(config Config) (*Node, error) {
	res, err := uniffiRustCallAsync[CreateError](
		FfiConverterCreateErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Node {
			return FfiConverterNodeINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_constructor_node_forwarder(FfiConverterConfigINSTANCE.Lower(config)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	if err == nil {
		return res, nil
	}

	return res, err
}

func NodeReceiver(config Config, handler DataHandler) (*Node, error) {
	res, err := uniffiRustCallAsync[CreateError](
		FfiConverterCreateErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Node {
			return FfiConverterNodeINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_constructor_node_receiver(FfiConverterConfigINSTANCE.Lower(config), FfiConverterDataHandlerINSTANCE.Lower(handler)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	if err == nil {
		return res, nil
	}

	return res, err
}

// Create a new streamplace client node.
func NodeSender(config Config) (*Node, error) {
	res, err := uniffiRustCallAsync[CreateError](
		FfiConverterCreateErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Node {
			return FfiConverterNodeINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_constructor_node_sender(FfiConverterConfigINSTANCE.Lower(config)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_pointer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_pointer(handle)
		},
	)

	if err == nil {
		return res, nil
	}

	return res, err
}

// Add tickets for remote peers
func (_self *Node) AddTickets(peers []string) error {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[JoinPeersError](
		FfiConverterJoinPeersErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_node_add_tickets(
			_pointer, FfiConverterSequenceStringINSTANCE.Lower(peers)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	if err == nil {
		return nil
	}

	return err
}

// Get a handle to the db to watch for changes locally or globally.
func (_self *Node) Db() *Db {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterDbINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_node_db(
			_pointer, _uniffiStatus)
	}))
}

// Join peers by their node tickets.
func (_self *Node) JoinPeers(peers []string) error {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[JoinPeersError](
		FfiConverterJoinPeersErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_node_join_peers(
			_pointer, FfiConverterSequenceStringINSTANCE.Lower(peers)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	if err == nil {
		return nil
	}

	return err
}

// Get this node's node ID.
func (_self *Node) NodeId() (*PublicKey, error) {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[PutError](
		FfiConverterPutErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *PublicKey {
			return FfiConverterPublicKeyINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_node_node_id(
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

	if err == nil {
		return res, nil
	}

	return res, err
}

// Get a handle to the write scope for this node.
//
// This is equivalent to calling `db.write(...)` with the secret key used to create the node.
func (_self *Node) NodeScope() *WriteScope {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterWriteScopeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_method_node_node_scope(
			_pointer, _uniffiStatus)
	}))
}

// Send a segment to all subscribers of the given stream.
func (_self *Node) SendSegment(key string, data []byte) error {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[PutError](
		FfiConverterPutErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_node_send_segment(
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

	if err == nil {
		return nil
	}

	return err
}

// Shutdown the node, including the streaming system and the metadata db.
func (_self *Node) Shutdown() error {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[ShutdownError](
		FfiConverterShutdownErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_node_shutdown(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	if err == nil {
		return nil
	}

	return err
}

// Subscribe to updates for a given stream from a remote node.
func (_self *Node) Subscribe(key string, remoteId *PublicKey) error {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[PutError](
		FfiConverterPutErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_node_subscribe(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterPublicKeyINSTANCE.Lower(remoteId)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	if err == nil {
		return nil
	}

	return err
}

// Get this node's ticket.
func (_self *Node) Ticket() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[PutError](
		FfiConverterPutErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_iroh_streamplace_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) string {
			return FfiConverterStringINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_node_ticket(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_rust_buffer(handle)
		},
	)

	if err == nil {
		return res, nil
	}

	return res, err
}

// Unsubscribe from updates for a given stream from a remote node.
func (_self *Node) Unsubscribe(key string, remoteId *PublicKey) error {
	_pointer := _self.ffiObject.incrementPointer("*Node")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[PutError](
		FfiConverterPutErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_node_unsubscribe(
			_pointer, FfiConverterStringINSTANCE.Lower(key), FfiConverterPublicKeyINSTANCE.Lower(remoteId)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	if err == nil {
		return nil
	}

	return err
}
func (object *Node) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterNode struct{}

var FfiConverterNodeINSTANCE = FfiConverterNode{}

func (c FfiConverterNode) Lift(pointer unsafe.Pointer) *Node {
	result := &Node{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_node(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_node(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Node).Destroy)
	return result
}

func (c FfiConverterNode) Read(reader io.Reader) *Node {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterNode) Lower(value *Node) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Node")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterNode) Write(writer io.Writer, value *Node) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerNode struct{}

func (_ FfiDestroyerNode) Destroy(value *Node) {
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
	// Express the PublicKey as a byte array
	AsVec() []byte
	// Returns true if the PublicKeys are equal
	Equal(other *PublicKey) bool
	// Convert to a base32 string limited to the first 10 bytes for a friendly string
	// representation of the key.
	FmtShort() string
}

// A public key.
//
// The key itself is just a 32 byte array, but a key has associated crypto
// information that is cached for performance reasons.
type PublicKey struct {
	ffiObject FfiObject
}

// Make a PublicKey from byte array
func PublicKeyFromBytes(bytes []byte) (*PublicKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[PublicKeyError](FfiConverterPublicKeyError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_constructor_publickey_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PublicKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPublicKeyINSTANCE.Lift(_uniffiRV), nil
	}
}

// Make a PublicKey from base32 string
func PublicKeyFromString(s string) (*PublicKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[PublicKeyError](FfiConverterPublicKeyError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_constructor_publickey_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PublicKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPublicKeyINSTANCE.Lift(_uniffiRV), nil
	}
}

// Express the PublicKey as a byte array
func (_self *PublicKey) AsVec() []byte {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_publickey_as_vec(
				_pointer, _uniffiStatus),
		}
	}))
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

// A response to a subscribe request.
//
// This can be used as a stream of [`SubscribeItem`]s.
type SubscribeResponseInterface interface {
	NextRaw() (*SubscribeItem, error)
}

// A response to a subscribe request.
//
// This can be used as a stream of [`SubscribeItem`]s.
type SubscribeResponse struct {
	ffiObject FfiObject
}

func (_self *SubscribeResponse) NextRaw() (*SubscribeItem, error) {
	_pointer := _self.ffiObject.incrementPointer("*SubscribeResponse")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[SubscribeNextError](
		FfiConverterSubscribeNextErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_iroh_streamplace_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) *SubscribeItem {
			return FfiConverterOptionalSubscribeItemINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_subscriberesponse_next_raw(
			_pointer),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_rust_buffer(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_rust_buffer(handle)
		},
	)

	if err == nil {
		return res, nil
	}

	return res, err
}

func (_self *SubscribeResponse) DebugString() string {
	_pointer := _self.ffiObject.incrementPointer("*SubscribeResponse")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_subscriberesponse_uniffi_trait_debug(
				_pointer, _uniffiStatus),
		}
	}))
}

func (object *SubscribeResponse) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSubscribeResponse struct{}

var FfiConverterSubscribeResponseINSTANCE = FfiConverterSubscribeResponse{}

func (c FfiConverterSubscribeResponse) Lift(pointer unsafe.Pointer) *SubscribeResponse {
	result := &SubscribeResponse{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_subscriberesponse(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_subscriberesponse(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*SubscribeResponse).Destroy)
	return result
}

func (c FfiConverterSubscribeResponse) Read(reader io.Reader) *SubscribeResponse {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSubscribeResponse) Lower(value *SubscribeResponse) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SubscribeResponse")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSubscribeResponse) Write(writer io.Writer, value *SubscribeResponse) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSubscribeResponse struct{}

func (_ FfiDestroyerSubscribeResponse) Destroy(value *SubscribeResponse) {
	value.Destroy()
}

// A write scope that can be used to put values into the database.
//
// The default write scope is available from the [`Node::node_scope`] method.
type WriteScopeInterface interface {
	Put(stream *[]byte, key []byte, value []byte) error
}

// A write scope that can be used to put values into the database.
//
// The default write scope is available from the [`Node::node_scope`] method.
type WriteScope struct {
	ffiObject FfiObject
}

func (_self *WriteScope) Put(stream *[]byte, key []byte, value []byte) error {
	_pointer := _self.ffiObject.incrementPointer("*WriteScope")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[PutError](
		FfiConverterPutErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_writescope_put(
			_pointer, FfiConverterOptionalBytesINSTANCE.Lower(stream), FfiConverterBytesINSTANCE.Lower(key), FfiConverterBytesINSTANCE.Lower(value)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_void(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_void(handle)
		},
	)

	if err == nil {
		return nil
	}

	return err
}
func (object *WriteScope) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWriteScope struct{}

var FfiConverterWriteScopeINSTANCE = FfiConverterWriteScope{}

func (c FfiConverterWriteScope) Lift(pointer unsafe.Pointer) *WriteScope {
	result := &WriteScope{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_writescope(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_writescope(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*WriteScope).Destroy)
	return result
}

func (c FfiConverterWriteScope) Read(reader io.Reader) *WriteScope {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWriteScope) Lower(value *WriteScope) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*WriteScope")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWriteScope) Write(writer io.Writer, value *WriteScope) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWriteScope struct{}

func (_ FfiDestroyerWriteScope) Destroy(value *WriteScope) {
	value.Destroy()
}

// Configuration for an iroh-streamplace node.
type Config struct {
	// An Ed25519 secret key as a 32 byte array.
	Key []byte
	// The gossip topic to use. Must be 32 bytes.
	//
	// You can use e.g. a BLAKE3 hash of a topic string here. This can be used
	// as a cheap way to have a shared secret - nodes that do not know the topic
	// cannot connect to the swarm.
	Topic []byte
	// Maximum duration to wait for sending a stream piece to a peer.
	MaxSendDuration time.Duration
	// Disable using relays, for tests.
	DisableRelay bool
}

func (r *Config) Destroy() {
	FfiDestroyerBytes{}.Destroy(r.Key)
	FfiDestroyerBytes{}.Destroy(r.Topic)
	FfiDestroyerDuration{}.Destroy(r.MaxSendDuration)
	FfiDestroyerBool{}.Destroy(r.DisableRelay)
}

type FfiConverterConfig struct{}

var FfiConverterConfigINSTANCE = FfiConverterConfig{}

func (c FfiConverterConfig) Lift(rb RustBufferI) Config {
	return LiftFromRustBuffer[Config](c, rb)
}

func (c FfiConverterConfig) Read(reader io.Reader) Config {
	return Config{
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterDurationINSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterConfig) Lower(value Config) C.RustBuffer {
	return LowerIntoRustBuffer[Config](c, value)
}

func (c FfiConverterConfig) Write(writer io.Writer, value Config) {
	FfiConverterBytesINSTANCE.Write(writer, value.Key)
	FfiConverterBytesINSTANCE.Write(writer, value.Topic)
	FfiConverterDurationINSTANCE.Write(writer, value.MaxSendDuration)
	FfiConverterBoolINSTANCE.Write(writer, value.DisableRelay)
}

type FfiDestroyerConfig struct{}

func (_ FfiDestroyerConfig) Destroy(value Config) {
	value.Destroy()
}

// An entry returned from the database.
type Entry struct {
	Scope     *PublicKey
	Stream    *[]byte
	Key       []byte
	Value     []byte
	Timestamp uint64
}

func (r *Entry) Destroy() {
	FfiDestroyerPublicKey{}.Destroy(r.Scope)
	FfiDestroyerOptionalBytes{}.Destroy(r.Stream)
	FfiDestroyerBytes{}.Destroy(r.Key)
	FfiDestroyerBytes{}.Destroy(r.Value)
	FfiDestroyerUint64{}.Destroy(r.Timestamp)
}

type FfiConverterEntry struct{}

var FfiConverterEntryINSTANCE = FfiConverterEntry{}

func (c FfiConverterEntry) Lift(rb RustBufferI) Entry {
	return LiftFromRustBuffer[Entry](c, rb)
}

func (c FfiConverterEntry) Read(reader io.Reader) Entry {
	return Entry{
		FfiConverterPublicKeyINSTANCE.Read(reader),
		FfiConverterOptionalBytesINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterEntry) Lower(value Entry) C.RustBuffer {
	return LowerIntoRustBuffer[Entry](c, value)
}

func (c FfiConverterEntry) Write(writer io.Writer, value Entry) {
	FfiConverterPublicKeyINSTANCE.Write(writer, value.Scope)
	FfiConverterOptionalBytesINSTANCE.Write(writer, value.Stream)
	FfiConverterBytesINSTANCE.Write(writer, value.Key)
	FfiConverterBytesINSTANCE.Write(writer, value.Value)
	FfiConverterUint64INSTANCE.Write(writer, value.Timestamp)
}

type FfiDestroyerEntry struct{}

func (_ FfiDestroyerEntry) Destroy(value Entry) {
	value.Destroy()
}

// Options for subscribing.
//
// `filter` specifies what to subscribe to.
// `mode` specifies whether to get current items, new items, or both.
type SubscribeOpts struct {
	Filter *Filter
	Mode   SubscribeMode
}

func (r *SubscribeOpts) Destroy() {
	FfiDestroyerFilter{}.Destroy(r.Filter)
	FfiDestroyerSubscribeMode{}.Destroy(r.Mode)
}

type FfiConverterSubscribeOpts struct{}

var FfiConverterSubscribeOptsINSTANCE = FfiConverterSubscribeOpts{}

func (c FfiConverterSubscribeOpts) Lift(rb RustBufferI) SubscribeOpts {
	return LiftFromRustBuffer[SubscribeOpts](c, rb)
}

func (c FfiConverterSubscribeOpts) Read(reader io.Reader) SubscribeOpts {
	return SubscribeOpts{
		FfiConverterFilterINSTANCE.Read(reader),
		FfiConverterSubscribeModeINSTANCE.Read(reader),
	}
}

func (c FfiConverterSubscribeOpts) Lower(value SubscribeOpts) C.RustBuffer {
	return LowerIntoRustBuffer[SubscribeOpts](c, value)
}

func (c FfiConverterSubscribeOpts) Write(writer io.Writer, value SubscribeOpts) {
	FfiConverterFilterINSTANCE.Write(writer, value.Filter)
	FfiConverterSubscribeModeINSTANCE.Write(writer, value.Mode)
}

type FfiDestroyerSubscribeOpts struct{}

func (_ FfiDestroyerSubscribeOpts) Destroy(value SubscribeOpts) {
	value.Destroy()
}

// Error creating a new database node.
type CreateError struct {
	err error
}

// Convience method to turn *CreateError into error
// Avoiding treating nil pointer as non nil error interface
func (err *CreateError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err CreateError) Error() string {
	return fmt.Sprintf("CreateError: %s", err.err.Error())
}

func (err CreateError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrCreateErrorPrivateKey = fmt.Errorf("CreateErrorPrivateKey")
var ErrCreateErrorTopic = fmt.Errorf("CreateErrorTopic")
var ErrCreateErrorBind = fmt.Errorf("CreateErrorBind")
var ErrCreateErrorSubscribe = fmt.Errorf("CreateErrorSubscribe")

// Variant structs
// The provided private key is invalid (not 32 bytes).
type CreateErrorPrivateKey struct {
	Size uint64
}

// The provided private key is invalid (not 32 bytes).
func NewCreateErrorPrivateKey(
	size uint64,
) *CreateError {
	return &CreateError{err: &CreateErrorPrivateKey{
		Size: size}}
}

func (e CreateErrorPrivateKey) destroy() {
	FfiDestroyerUint64{}.Destroy(e.Size)
}

func (err CreateErrorPrivateKey) Error() string {
	return fmt.Sprint("PrivateKey",
		": ",

		"Size=",
		err.Size,
	)
}

func (self CreateErrorPrivateKey) Is(target error) bool {
	return target == ErrCreateErrorPrivateKey
}

// The provided gossip topic is invalid (not 32 bytes).
type CreateErrorTopic struct {
	Size uint64
}

// The provided gossip topic is invalid (not 32 bytes).
func NewCreateErrorTopic(
	size uint64,
) *CreateError {
	return &CreateError{err: &CreateErrorTopic{
		Size: size}}
}

func (e CreateErrorTopic) destroy() {
	FfiDestroyerUint64{}.Destroy(e.Size)
}

func (err CreateErrorTopic) Error() string {
	return fmt.Sprint("Topic",
		": ",

		"Size=",
		err.Size,
	)
}

func (self CreateErrorTopic) Is(target error) bool {
	return target == ErrCreateErrorTopic
}

// Failed to bind the iroh endpoint.
type CreateErrorBind struct {
	Message string
}

// Failed to bind the iroh endpoint.
func NewCreateErrorBind(
	message string,
) *CreateError {
	return &CreateError{err: &CreateErrorBind{
		Message: message}}
}

func (e CreateErrorBind) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err CreateErrorBind) Error() string {
	return fmt.Sprint("Bind",
		": ",

		"Message=",
		err.Message,
	)
}

func (self CreateErrorBind) Is(target error) bool {
	return target == ErrCreateErrorBind
}

// Failed to subscribe to the gossip topic.
type CreateErrorSubscribe struct {
	Message string
}

// Failed to subscribe to the gossip topic.
func NewCreateErrorSubscribe(
	message string,
) *CreateError {
	return &CreateError{err: &CreateErrorSubscribe{
		Message: message}}
}

func (e CreateErrorSubscribe) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err CreateErrorSubscribe) Error() string {
	return fmt.Sprint("Subscribe",
		": ",

		"Message=",
		err.Message,
	)
}

func (self CreateErrorSubscribe) Is(target error) bool {
	return target == ErrCreateErrorSubscribe
}

type FfiConverterCreateError struct{}

var FfiConverterCreateErrorINSTANCE = FfiConverterCreateError{}

func (c FfiConverterCreateError) Lift(eb RustBufferI) *CreateError {
	return LiftFromRustBuffer[*CreateError](c, eb)
}

func (c FfiConverterCreateError) Lower(value *CreateError) C.RustBuffer {
	return LowerIntoRustBuffer[*CreateError](c, value)
}

func (c FfiConverterCreateError) Read(reader io.Reader) *CreateError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &CreateError{&CreateErrorPrivateKey{
			Size: FfiConverterUint64INSTANCE.Read(reader),
		}}
	case 2:
		return &CreateError{&CreateErrorTopic{
			Size: FfiConverterUint64INSTANCE.Read(reader),
		}}
	case 3:
		return &CreateError{&CreateErrorBind{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 4:
		return &CreateError{&CreateErrorSubscribe{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterCreateError.Read()", errorID))
	}
}

func (c FfiConverterCreateError) Write(writer io.Writer, value *CreateError) {
	switch variantValue := value.err.(type) {
	case *CreateErrorPrivateKey:
		writeInt32(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.Size)
	case *CreateErrorTopic:
		writeInt32(writer, 2)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.Size)
	case *CreateErrorBind:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	case *CreateErrorSubscribe:
		writeInt32(writer, 4)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterCreateError.Write", value))
	}
}

type FfiDestroyerCreateError struct{}

func (_ FfiDestroyerCreateError) Destroy(value *CreateError) {
	switch variantValue := value.err.(type) {
	case CreateErrorPrivateKey:
		variantValue.destroy()
	case CreateErrorTopic:
		variantValue.destroy()
	case CreateErrorBind:
		variantValue.destroy()
	case CreateErrorSubscribe:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerCreateError.Destroy", value))
	}
}

// Error joining peers.
type JoinPeersError struct {
	err error
}

// Convience method to turn *JoinPeersError into error
// Avoiding treating nil pointer as non nil error interface
func (err *JoinPeersError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err JoinPeersError) Error() string {
	return fmt.Sprintf("JoinPeersError: %s", err.err.Error())
}

func (err JoinPeersError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrJoinPeersErrorTicket = fmt.Errorf("JoinPeersErrorTicket")
var ErrJoinPeersErrorIrpc = fmt.Errorf("JoinPeersErrorIrpc")

// Variant structs
// Failed to parse a provided iroh node ticket.
type JoinPeersErrorTicket struct {
	Message string
}

// Failed to parse a provided iroh node ticket.
func NewJoinPeersErrorTicket(
	message string,
) *JoinPeersError {
	return &JoinPeersError{err: &JoinPeersErrorTicket{
		Message: message}}
}

func (e JoinPeersErrorTicket) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err JoinPeersErrorTicket) Error() string {
	return fmt.Sprint("Ticket",
		": ",

		"Message=",
		err.Message,
	)
}

func (self JoinPeersErrorTicket) Is(target error) bool {
	return target == ErrJoinPeersErrorTicket
}

// Error during the join peers operation.
type JoinPeersErrorIrpc struct {
	Message string
}

// Error during the join peers operation.
func NewJoinPeersErrorIrpc(
	message string,
) *JoinPeersError {
	return &JoinPeersError{err: &JoinPeersErrorIrpc{
		Message: message}}
}

func (e JoinPeersErrorIrpc) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err JoinPeersErrorIrpc) Error() string {
	return fmt.Sprint("Irpc",
		": ",

		"Message=",
		err.Message,
	)
}

func (self JoinPeersErrorIrpc) Is(target error) bool {
	return target == ErrJoinPeersErrorIrpc
}

type FfiConverterJoinPeersError struct{}

var FfiConverterJoinPeersErrorINSTANCE = FfiConverterJoinPeersError{}

func (c FfiConverterJoinPeersError) Lift(eb RustBufferI) *JoinPeersError {
	return LiftFromRustBuffer[*JoinPeersError](c, eb)
}

func (c FfiConverterJoinPeersError) Lower(value *JoinPeersError) C.RustBuffer {
	return LowerIntoRustBuffer[*JoinPeersError](c, value)
}

func (c FfiConverterJoinPeersError) Read(reader io.Reader) *JoinPeersError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &JoinPeersError{&JoinPeersErrorTicket{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &JoinPeersError{&JoinPeersErrorIrpc{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterJoinPeersError.Read()", errorID))
	}
}

func (c FfiConverterJoinPeersError) Write(writer io.Writer, value *JoinPeersError) {
	switch variantValue := value.err.(type) {
	case *JoinPeersErrorTicket:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	case *JoinPeersErrorIrpc:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterJoinPeersError.Write", value))
	}
}

type FfiDestroyerJoinPeersError struct{}

func (_ FfiDestroyerJoinPeersError) Destroy(value *JoinPeersError) {
	switch variantValue := value.err.(type) {
	case JoinPeersErrorTicket:
		variantValue.destroy()
	case JoinPeersErrorIrpc:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerJoinPeersError.Destroy", value))
	}
}

// Error when converting from ffi NodeAddr to iroh::NodeAddr
type NodeAddrError struct {
	err error
}

// Convience method to turn *NodeAddrError into error
// Avoiding treating nil pointer as non nil error interface
func (err *NodeAddrError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err NodeAddrError) Error() string {
	return fmt.Sprintf("NodeAddrError: %s", err.err.Error())
}

func (err NodeAddrError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrNodeAddrErrorInvalidUrl = fmt.Errorf("NodeAddrErrorInvalidUrl")
var ErrNodeAddrErrorInvalidNetworkAddress = fmt.Errorf("NodeAddrErrorInvalidNetworkAddress")

// Variant structs
type NodeAddrErrorInvalidUrl struct {
	message string
}

func NewNodeAddrErrorInvalidUrl() *NodeAddrError {
	return &NodeAddrError{err: &NodeAddrErrorInvalidUrl{}}
}

func (e NodeAddrErrorInvalidUrl) destroy() {
}

func (err NodeAddrErrorInvalidUrl) Error() string {
	return fmt.Sprintf("InvalidUrl: %s", err.message)
}

func (self NodeAddrErrorInvalidUrl) Is(target error) bool {
	return target == ErrNodeAddrErrorInvalidUrl
}

type NodeAddrErrorInvalidNetworkAddress struct {
	message string
}

func NewNodeAddrErrorInvalidNetworkAddress() *NodeAddrError {
	return &NodeAddrError{err: &NodeAddrErrorInvalidNetworkAddress{}}
}

func (e NodeAddrErrorInvalidNetworkAddress) destroy() {
}

func (err NodeAddrErrorInvalidNetworkAddress) Error() string {
	return fmt.Sprintf("InvalidNetworkAddress: %s", err.message)
}

func (self NodeAddrErrorInvalidNetworkAddress) Is(target error) bool {
	return target == ErrNodeAddrErrorInvalidNetworkAddress
}

type FfiConverterNodeAddrError struct{}

var FfiConverterNodeAddrErrorINSTANCE = FfiConverterNodeAddrError{}

func (c FfiConverterNodeAddrError) Lift(eb RustBufferI) *NodeAddrError {
	return LiftFromRustBuffer[*NodeAddrError](c, eb)
}

func (c FfiConverterNodeAddrError) Lower(value *NodeAddrError) C.RustBuffer {
	return LowerIntoRustBuffer[*NodeAddrError](c, value)
}

func (c FfiConverterNodeAddrError) Read(reader io.Reader) *NodeAddrError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &NodeAddrError{&NodeAddrErrorInvalidUrl{message}}
	case 2:
		return &NodeAddrError{&NodeAddrErrorInvalidNetworkAddress{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterNodeAddrError.Read()", errorID))
	}

}

func (c FfiConverterNodeAddrError) Write(writer io.Writer, value *NodeAddrError) {
	switch variantValue := value.err.(type) {
	case *NodeAddrErrorInvalidUrl:
		writeInt32(writer, 1)
	case *NodeAddrErrorInvalidNetworkAddress:
		writeInt32(writer, 2)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterNodeAddrError.Write", value))
	}
}

type FfiDestroyerNodeAddrError struct{}

func (_ FfiDestroyerNodeAddrError) Destroy(value *NodeAddrError) {
	switch variantValue := value.err.(type) {
	case NodeAddrErrorInvalidUrl:
		variantValue.destroy()
	case NodeAddrErrorInvalidNetworkAddress:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerNodeAddrError.Destroy", value))
	}
}

// Error joining peers.
type ParseError struct {
	err error
}

// Convience method to turn *ParseError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ParseError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ParseError) Error() string {
	return fmt.Sprintf("ParseError: %s", err.err.Error())
}

func (err ParseError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrParseErrorTicket = fmt.Errorf("ParseErrorTicket")

// Variant structs
// Failed to parse a provided iroh node ticket.
type ParseErrorTicket struct {
	Message string
}

// Failed to parse a provided iroh node ticket.
func NewParseErrorTicket(
	message string,
) *ParseError {
	return &ParseError{err: &ParseErrorTicket{
		Message: message}}
}

func (e ParseErrorTicket) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err ParseErrorTicket) Error() string {
	return fmt.Sprint("Ticket",
		": ",

		"Message=",
		err.Message,
	)
}

func (self ParseErrorTicket) Is(target error) bool {
	return target == ErrParseErrorTicket
}

type FfiConverterParseError struct{}

var FfiConverterParseErrorINSTANCE = FfiConverterParseError{}

func (c FfiConverterParseError) Lift(eb RustBufferI) *ParseError {
	return LiftFromRustBuffer[*ParseError](c, eb)
}

func (c FfiConverterParseError) Lower(value *ParseError) C.RustBuffer {
	return LowerIntoRustBuffer[*ParseError](c, value)
}

func (c FfiConverterParseError) Read(reader io.Reader) *ParseError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &ParseError{&ParseErrorTicket{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterParseError.Read()", errorID))
	}
}

func (c FfiConverterParseError) Write(writer io.Writer, value *ParseError) {
	switch variantValue := value.err.(type) {
	case *ParseErrorTicket:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterParseError.Write", value))
	}
}

type FfiDestroyerParseError struct{}

func (_ FfiDestroyerParseError) Destroy(value *ParseError) {
	switch variantValue := value.err.(type) {
	case ParseErrorTicket:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerParseError.Destroy", value))
	}
}

type PublicKeyError struct {
	err error
}

// Convience method to turn *PublicKeyError into error
// Avoiding treating nil pointer as non nil error interface
func (err *PublicKeyError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err PublicKeyError) Error() string {
	return fmt.Sprintf("PublicKeyError: %s", err.err.Error())
}

func (err PublicKeyError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrPublicKeyErrorLength = fmt.Errorf("PublicKeyErrorLength")
var ErrPublicKeyErrorInvalid = fmt.Errorf("PublicKeyErrorInvalid")

// Variant structs
type PublicKeyErrorLength struct {
	Size uint64
}

func NewPublicKeyErrorLength(
	size uint64,
) *PublicKeyError {
	return &PublicKeyError{err: &PublicKeyErrorLength{
		Size: size}}
}

func (e PublicKeyErrorLength) destroy() {
	FfiDestroyerUint64{}.Destroy(e.Size)
}

func (err PublicKeyErrorLength) Error() string {
	return fmt.Sprint("Length",
		": ",

		"Size=",
		err.Size,
	)
}

func (self PublicKeyErrorLength) Is(target error) bool {
	return target == ErrPublicKeyErrorLength
}

type PublicKeyErrorInvalid struct {
	Message string
}

func NewPublicKeyErrorInvalid(
	message string,
) *PublicKeyError {
	return &PublicKeyError{err: &PublicKeyErrorInvalid{
		Message: message}}
}

func (e PublicKeyErrorInvalid) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err PublicKeyErrorInvalid) Error() string {
	return fmt.Sprint("Invalid",
		": ",

		"Message=",
		err.Message,
	)
}

func (self PublicKeyErrorInvalid) Is(target error) bool {
	return target == ErrPublicKeyErrorInvalid
}

type FfiConverterPublicKeyError struct{}

var FfiConverterPublicKeyErrorINSTANCE = FfiConverterPublicKeyError{}

func (c FfiConverterPublicKeyError) Lift(eb RustBufferI) *PublicKeyError {
	return LiftFromRustBuffer[*PublicKeyError](c, eb)
}

func (c FfiConverterPublicKeyError) Lower(value *PublicKeyError) C.RustBuffer {
	return LowerIntoRustBuffer[*PublicKeyError](c, value)
}

func (c FfiConverterPublicKeyError) Read(reader io.Reader) *PublicKeyError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &PublicKeyError{&PublicKeyErrorLength{
			Size: FfiConverterUint64INSTANCE.Read(reader),
		}}
	case 2:
		return &PublicKeyError{&PublicKeyErrorInvalid{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterPublicKeyError.Read()", errorID))
	}
}

func (c FfiConverterPublicKeyError) Write(writer io.Writer, value *PublicKeyError) {
	switch variantValue := value.err.(type) {
	case *PublicKeyErrorLength:
		writeInt32(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.Size)
	case *PublicKeyErrorInvalid:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterPublicKeyError.Write", value))
	}
}

type FfiDestroyerPublicKeyError struct{}

func (_ FfiDestroyerPublicKeyError) Destroy(value *PublicKeyError) {
	switch variantValue := value.err.(type) {
	case PublicKeyErrorLength:
		variantValue.destroy()
	case PublicKeyErrorInvalid:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerPublicKeyError.Destroy", value))
	}
}

// Error putting a value into the database.
type PutError struct {
	err error
}

// Convience method to turn *PutError into error
// Avoiding treating nil pointer as non nil error interface
func (err *PutError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err PutError) Error() string {
	return fmt.Sprintf("PutError: %s", err.err.Error())
}

func (err PutError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrPutErrorIrpc = fmt.Errorf("PutErrorIrpc")

// Variant structs
// Error during the put operation.
type PutErrorIrpc struct {
	Message string
}

// Error during the put operation.
func NewPutErrorIrpc(
	message string,
) *PutError {
	return &PutError{err: &PutErrorIrpc{
		Message: message}}
}

func (e PutErrorIrpc) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err PutErrorIrpc) Error() string {
	return fmt.Sprint("Irpc",
		": ",

		"Message=",
		err.Message,
	)
}

func (self PutErrorIrpc) Is(target error) bool {
	return target == ErrPutErrorIrpc
}

type FfiConverterPutError struct{}

var FfiConverterPutErrorINSTANCE = FfiConverterPutError{}

func (c FfiConverterPutError) Lift(eb RustBufferI) *PutError {
	return LiftFromRustBuffer[*PutError](c, eb)
}

func (c FfiConverterPutError) Lower(value *PutError) C.RustBuffer {
	return LowerIntoRustBuffer[*PutError](c, value)
}

func (c FfiConverterPutError) Read(reader io.Reader) *PutError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &PutError{&PutErrorIrpc{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterPutError.Read()", errorID))
	}
}

func (c FfiConverterPutError) Write(writer io.Writer, value *PutError) {
	switch variantValue := value.err.(type) {
	case *PutErrorIrpc:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterPutError.Write", value))
	}
}

type FfiDestroyerPutError struct{}

func (_ FfiDestroyerPutError) Destroy(value *PutError) {
	switch variantValue := value.err.(type) {
	case PutErrorIrpc:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerPutError.Destroy", value))
	}
}

type SpError struct {
	err error
}

// Convience method to turn *SpError into error
// Avoiding treating nil pointer as non nil error interface
func (err *SpError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err SpError) Error() string {
	return fmt.Sprintf("SpError: %s", err.err.Error())
}

func (err SpError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrSpErrorNoCertificateChainFound = fmt.Errorf("SpErrorNoCertificateChainFound")
var ErrSpErrorC2paError = fmt.Errorf("SpErrorC2paError")

// Variant structs
type SpErrorNoCertificateChainFound struct {
	message string
}

func NewSpErrorNoCertificateChainFound() *SpError {
	return &SpError{err: &SpErrorNoCertificateChainFound{}}
}

func (e SpErrorNoCertificateChainFound) destroy() {
}

func (err SpErrorNoCertificateChainFound) Error() string {
	return fmt.Sprintf("NoCertificateChainFound: %s", err.message)
}

func (self SpErrorNoCertificateChainFound) Is(target error) bool {
	return target == ErrSpErrorNoCertificateChainFound
}

type SpErrorC2paError struct {
	message string
}

func NewSpErrorC2paError() *SpError {
	return &SpError{err: &SpErrorC2paError{}}
}

func (e SpErrorC2paError) destroy() {
}

func (err SpErrorC2paError) Error() string {
	return fmt.Sprintf("C2paError: %s", err.message)
}

func (self SpErrorC2paError) Is(target error) bool {
	return target == ErrSpErrorC2paError
}

type FfiConverterSpError struct{}

var FfiConverterSpErrorINSTANCE = FfiConverterSpError{}

func (c FfiConverterSpError) Lift(eb RustBufferI) *SpError {
	return LiftFromRustBuffer[*SpError](c, eb)
}

func (c FfiConverterSpError) Lower(value *SpError) C.RustBuffer {
	return LowerIntoRustBuffer[*SpError](c, value)
}

func (c FfiConverterSpError) Read(reader io.Reader) *SpError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &SpError{&SpErrorNoCertificateChainFound{message}}
	case 2:
		return &SpError{&SpErrorC2paError{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterSpError.Read()", errorID))
	}

}

func (c FfiConverterSpError) Write(writer io.Writer, value *SpError) {
	switch variantValue := value.err.(type) {
	case *SpErrorNoCertificateChainFound:
		writeInt32(writer, 1)
	case *SpErrorC2paError:
		writeInt32(writer, 2)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterSpError.Write", value))
	}
}

type FfiDestroyerSpError struct{}

func (_ FfiDestroyerSpError) Destroy(value *SpError) {
	switch variantValue := value.err.(type) {
	case SpErrorNoCertificateChainFound:
		variantValue.destroy()
	case SpErrorC2paError:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerSpError.Destroy", value))
	}
}

// Error shutting down the database.
//
// This can occur if the db is already shut down or if there is an internal error.
type ShutdownError struct {
	err error
}

// Convience method to turn *ShutdownError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ShutdownError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ShutdownError) Error() string {
	return fmt.Sprintf("ShutdownError: %s", err.err.Error())
}

func (err ShutdownError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrShutdownErrorIrpc = fmt.Errorf("ShutdownErrorIrpc")

// Variant structs
// Error during the shutdown operation.
type ShutdownErrorIrpc struct {
	Message string
}

// Error during the shutdown operation.
func NewShutdownErrorIrpc(
	message string,
) *ShutdownError {
	return &ShutdownError{err: &ShutdownErrorIrpc{
		Message: message}}
}

func (e ShutdownErrorIrpc) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err ShutdownErrorIrpc) Error() string {
	return fmt.Sprint("Irpc",
		": ",

		"Message=",
		err.Message,
	)
}

func (self ShutdownErrorIrpc) Is(target error) bool {
	return target == ErrShutdownErrorIrpc
}

type FfiConverterShutdownError struct{}

var FfiConverterShutdownErrorINSTANCE = FfiConverterShutdownError{}

func (c FfiConverterShutdownError) Lift(eb RustBufferI) *ShutdownError {
	return LiftFromRustBuffer[*ShutdownError](c, eb)
}

func (c FfiConverterShutdownError) Lower(value *ShutdownError) C.RustBuffer {
	return LowerIntoRustBuffer[*ShutdownError](c, value)
}

func (c FfiConverterShutdownError) Read(reader io.Reader) *ShutdownError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &ShutdownError{&ShutdownErrorIrpc{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterShutdownError.Read()", errorID))
	}
}

func (c FfiConverterShutdownError) Write(writer io.Writer, value *ShutdownError) {
	switch variantValue := value.err.(type) {
	case *ShutdownErrorIrpc:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterShutdownError.Write", value))
	}
}

type FfiDestroyerShutdownError struct{}

func (_ FfiDestroyerShutdownError) Destroy(value *ShutdownError) {
	switch variantValue := value.err.(type) {
	case ShutdownErrorIrpc:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerShutdownError.Destroy", value))
	}
}

type StreamFilter interface {
	Destroy()
}
type StreamFilterAll struct {
}

func (e StreamFilterAll) Destroy() {
}

type StreamFilterGlobal struct {
}

func (e StreamFilterGlobal) Destroy() {
}

type StreamFilterStream struct {
	Field0 []byte
}

func (e StreamFilterStream) Destroy() {
	FfiDestroyerBytes{}.Destroy(e.Field0)
}

type FfiConverterStreamFilter struct{}

var FfiConverterStreamFilterINSTANCE = FfiConverterStreamFilter{}

func (c FfiConverterStreamFilter) Lift(rb RustBufferI) StreamFilter {
	return LiftFromRustBuffer[StreamFilter](c, rb)
}

func (c FfiConverterStreamFilter) Lower(value StreamFilter) C.RustBuffer {
	return LowerIntoRustBuffer[StreamFilter](c, value)
}
func (FfiConverterStreamFilter) Read(reader io.Reader) StreamFilter {
	id := readInt32(reader)
	switch id {
	case 1:
		return StreamFilterAll{}
	case 2:
		return StreamFilterGlobal{}
	case 3:
		return StreamFilterStream{
			FfiConverterBytesINSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterStreamFilter.Read()", id))
	}
}

func (FfiConverterStreamFilter) Write(writer io.Writer, value StreamFilter) {
	switch variant_value := value.(type) {
	case StreamFilterAll:
		writeInt32(writer, 1)
	case StreamFilterGlobal:
		writeInt32(writer, 2)
	case StreamFilterStream:
		writeInt32(writer, 3)
		FfiConverterBytesINSTANCE.Write(writer, variant_value.Field0)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterStreamFilter.Write", value))
	}
}

type FfiDestroyerStreamFilter struct{}

func (_ FfiDestroyerStreamFilter) Destroy(value StreamFilter) {
	value.Destroy()
}

// An item returned from a subscription.
type SubscribeItem interface {
	Destroy()
}
type SubscribeItemEntry struct {
	Scope     *PublicKey
	Stream    *[]byte
	Key       []byte
	Value     []byte
	Timestamp uint64
}

func (e SubscribeItemEntry) Destroy() {
	FfiDestroyerPublicKey{}.Destroy(e.Scope)
	FfiDestroyerOptionalBytes{}.Destroy(e.Stream)
	FfiDestroyerBytes{}.Destroy(e.Key)
	FfiDestroyerBytes{}.Destroy(e.Value)
	FfiDestroyerUint64{}.Destroy(e.Timestamp)
}

type SubscribeItemCurrentDone struct {
}

func (e SubscribeItemCurrentDone) Destroy() {
}

type SubscribeItemExpired struct {
	Scope     *PublicKey
	Stream    *[]byte
	Key       []byte
	Timestamp uint64
}

func (e SubscribeItemExpired) Destroy() {
	FfiDestroyerPublicKey{}.Destroy(e.Scope)
	FfiDestroyerOptionalBytes{}.Destroy(e.Stream)
	FfiDestroyerBytes{}.Destroy(e.Key)
	FfiDestroyerUint64{}.Destroy(e.Timestamp)
}

type SubscribeItemOther struct {
}

func (e SubscribeItemOther) Destroy() {
}

type FfiConverterSubscribeItem struct{}

var FfiConverterSubscribeItemINSTANCE = FfiConverterSubscribeItem{}

func (c FfiConverterSubscribeItem) Lift(rb RustBufferI) SubscribeItem {
	return LiftFromRustBuffer[SubscribeItem](c, rb)
}

func (c FfiConverterSubscribeItem) Lower(value SubscribeItem) C.RustBuffer {
	return LowerIntoRustBuffer[SubscribeItem](c, value)
}
func (FfiConverterSubscribeItem) Read(reader io.Reader) SubscribeItem {
	id := readInt32(reader)
	switch id {
	case 1:
		return SubscribeItemEntry{
			FfiConverterPublicKeyINSTANCE.Read(reader),
			FfiConverterOptionalBytesINSTANCE.Read(reader),
			FfiConverterBytesINSTANCE.Read(reader),
			FfiConverterBytesINSTANCE.Read(reader),
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 2:
		return SubscribeItemCurrentDone{}
	case 3:
		return SubscribeItemExpired{
			FfiConverterPublicKeyINSTANCE.Read(reader),
			FfiConverterOptionalBytesINSTANCE.Read(reader),
			FfiConverterBytesINSTANCE.Read(reader),
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 4:
		return SubscribeItemOther{}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterSubscribeItem.Read()", id))
	}
}

func (FfiConverterSubscribeItem) Write(writer io.Writer, value SubscribeItem) {
	switch variant_value := value.(type) {
	case SubscribeItemEntry:
		writeInt32(writer, 1)
		FfiConverterPublicKeyINSTANCE.Write(writer, variant_value.Scope)
		FfiConverterOptionalBytesINSTANCE.Write(writer, variant_value.Stream)
		FfiConverterBytesINSTANCE.Write(writer, variant_value.Key)
		FfiConverterBytesINSTANCE.Write(writer, variant_value.Value)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Timestamp)
	case SubscribeItemCurrentDone:
		writeInt32(writer, 2)
	case SubscribeItemExpired:
		writeInt32(writer, 3)
		FfiConverterPublicKeyINSTANCE.Write(writer, variant_value.Scope)
		FfiConverterOptionalBytesINSTANCE.Write(writer, variant_value.Stream)
		FfiConverterBytesINSTANCE.Write(writer, variant_value.Key)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Timestamp)
	case SubscribeItemOther:
		writeInt32(writer, 4)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterSubscribeItem.Write", value))
	}
}

type FfiDestroyerSubscribeItem struct{}

func (_ FfiDestroyerSubscribeItem) Destroy(value SubscribeItem) {
	value.Destroy()
}

// Subscription mode for key-value subscriptions.
type SubscribeMode uint

const (
	SubscribeModeCurrent SubscribeMode = 1
	SubscribeModeFuture  SubscribeMode = 2
	SubscribeModeBoth    SubscribeMode = 3
)

type FfiConverterSubscribeMode struct{}

var FfiConverterSubscribeModeINSTANCE = FfiConverterSubscribeMode{}

func (c FfiConverterSubscribeMode) Lift(rb RustBufferI) SubscribeMode {
	return LiftFromRustBuffer[SubscribeMode](c, rb)
}

func (c FfiConverterSubscribeMode) Lower(value SubscribeMode) C.RustBuffer {
	return LowerIntoRustBuffer[SubscribeMode](c, value)
}
func (FfiConverterSubscribeMode) Read(reader io.Reader) SubscribeMode {
	id := readInt32(reader)
	return SubscribeMode(id)
}

func (FfiConverterSubscribeMode) Write(writer io.Writer, value SubscribeMode) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerSubscribeMode struct{}

func (_ FfiDestroyerSubscribeMode) Destroy(value SubscribeMode) {
}

// Error getting the next item from a subscription.
type SubscribeNextError struct {
	err error
}

// Convience method to turn *SubscribeNextError into error
// Avoiding treating nil pointer as non nil error interface
func (err *SubscribeNextError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err SubscribeNextError) Error() string {
	return fmt.Sprintf("SubscribeNextError: %s", err.err.Error())
}

func (err SubscribeNextError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrSubscribeNextErrorIrpc = fmt.Errorf("SubscribeNextErrorIrpc")

// Variant structs
// Error during the subscribe next operation.
type SubscribeNextErrorIrpc struct {
	Message string
}

// Error during the subscribe next operation.
func NewSubscribeNextErrorIrpc(
	message string,
) *SubscribeNextError {
	return &SubscribeNextError{err: &SubscribeNextErrorIrpc{
		Message: message}}
}

func (e SubscribeNextErrorIrpc) destroy() {
	FfiDestroyerString{}.Destroy(e.Message)
}

func (err SubscribeNextErrorIrpc) Error() string {
	return fmt.Sprint("Irpc",
		": ",

		"Message=",
		err.Message,
	)
}

func (self SubscribeNextErrorIrpc) Is(target error) bool {
	return target == ErrSubscribeNextErrorIrpc
}

type FfiConverterSubscribeNextError struct{}

var FfiConverterSubscribeNextErrorINSTANCE = FfiConverterSubscribeNextError{}

func (c FfiConverterSubscribeNextError) Lift(eb RustBufferI) *SubscribeNextError {
	return LiftFromRustBuffer[*SubscribeNextError](c, eb)
}

func (c FfiConverterSubscribeNextError) Lower(value *SubscribeNextError) C.RustBuffer {
	return LowerIntoRustBuffer[*SubscribeNextError](c, value)
}

func (c FfiConverterSubscribeNextError) Read(reader io.Reader) *SubscribeNextError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &SubscribeNextError{&SubscribeNextErrorIrpc{
			Message: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterSubscribeNextError.Read()", errorID))
	}
}

func (c FfiConverterSubscribeNextError) Write(writer io.Writer, value *SubscribeNextError) {
	switch variantValue := value.err.(type) {
	case *SubscribeNextErrorIrpc:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Message)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterSubscribeNextError.Write", value))
	}
}

type FfiDestroyerSubscribeNextError struct{}

func (_ FfiDestroyerSubscribeNextError) Destroy(value *SubscribeNextError) {
	switch variantValue := value.err.(type) {
	case SubscribeNextErrorIrpc:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerSubscribeNextError.Destroy", value))
	}
}

// Error when converting from ffi NodeAddr to iroh::NodeAddr
type TicketError struct {
	err error
}

// Convience method to turn *TicketError into error
// Avoiding treating nil pointer as non nil error interface
func (err *TicketError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err TicketError) Error() string {
	return fmt.Sprintf("TicketError: %s", err.err.Error())
}

func (err TicketError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrTicketErrorParseError = fmt.Errorf("TicketErrorParseError")

// Variant structs
type TicketErrorParseError struct {
	message string
}

func NewTicketErrorParseError() *TicketError {
	return &TicketError{err: &TicketErrorParseError{}}
}

func (e TicketErrorParseError) destroy() {
}

func (err TicketErrorParseError) Error() string {
	return fmt.Sprintf("ParseError: %s", err.message)
}

func (self TicketErrorParseError) Is(target error) bool {
	return target == ErrTicketErrorParseError
}

type FfiConverterTicketError struct{}

var FfiConverterTicketErrorINSTANCE = FfiConverterTicketError{}

func (c FfiConverterTicketError) Lift(eb RustBufferI) *TicketError {
	return LiftFromRustBuffer[*TicketError](c, eb)
}

func (c FfiConverterTicketError) Lower(value *TicketError) C.RustBuffer {
	return LowerIntoRustBuffer[*TicketError](c, value)
}

func (c FfiConverterTicketError) Read(reader io.Reader) *TicketError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &TicketError{&TicketErrorParseError{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterTicketError.Read()", errorID))
	}

}

func (c FfiConverterTicketError) Write(writer io.Writer, value *TicketError) {
	switch variantValue := value.err.(type) {
	case *TicketErrorParseError:
		writeInt32(writer, 1)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterTicketError.Write", value))
	}
}

type FfiDestroyerTicketError struct{}

func (_ FfiDestroyerTicketError) Destroy(value *TicketError) {
	switch variantValue := value.err.(type) {
	case TicketErrorParseError:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerTicketError.Destroy", value))
	}
}

// A bound on time for filtering.
type TimeBound interface {
	Destroy()
}
type TimeBoundUnbounded struct {
}

func (e TimeBoundUnbounded) Destroy() {
}

type TimeBoundIncluded struct {
	Field0 uint64
}

func (e TimeBoundIncluded) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.Field0)
}

type TimeBoundExcluded struct {
	Field0 uint64
}

func (e TimeBoundExcluded) Destroy() {
	FfiDestroyerUint64{}.Destroy(e.Field0)
}

type FfiConverterTimeBound struct{}

var FfiConverterTimeBoundINSTANCE = FfiConverterTimeBound{}

func (c FfiConverterTimeBound) Lift(rb RustBufferI) TimeBound {
	return LiftFromRustBuffer[TimeBound](c, rb)
}

func (c FfiConverterTimeBound) Lower(value TimeBound) C.RustBuffer {
	return LowerIntoRustBuffer[TimeBound](c, value)
}
func (FfiConverterTimeBound) Read(reader io.Reader) TimeBound {
	id := readInt32(reader)
	switch id {
	case 1:
		return TimeBoundUnbounded{}
	case 2:
		return TimeBoundIncluded{
			FfiConverterUint64INSTANCE.Read(reader),
		}
	case 3:
		return TimeBoundExcluded{
			FfiConverterUint64INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterTimeBound.Read()", id))
	}
}

func (FfiConverterTimeBound) Write(writer io.Writer, value TimeBound) {
	switch variant_value := value.(type) {
	case TimeBoundUnbounded:
		writeInt32(writer, 1)
	case TimeBoundIncluded:
		writeInt32(writer, 2)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Field0)
	case TimeBoundExcluded:
		writeInt32(writer, 3)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.Field0)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterTimeBound.Write", value))
	}
}

type FfiDestroyerTimeBound struct{}

func (_ FfiDestroyerTimeBound) Destroy(value TimeBound) {
	value.Destroy()
}

// Error getting the next item from a subscription.
type WriteError struct {
	err error
}

// Convience method to turn *WriteError into error
// Avoiding treating nil pointer as non nil error interface
func (err *WriteError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err WriteError) Error() string {
	return fmt.Sprintf("WriteError: %s", err.err.Error())
}

func (err WriteError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrWriteErrorPrivateKeySize = fmt.Errorf("WriteErrorPrivateKeySize")

// Variant structs
// The provided private key is invalid (not 32 bytes).
type WriteErrorPrivateKeySize struct {
	Size uint64
}

// The provided private key is invalid (not 32 bytes).
func NewWriteErrorPrivateKeySize(
	size uint64,
) *WriteError {
	return &WriteError{err: &WriteErrorPrivateKeySize{
		Size: size}}
}

func (e WriteErrorPrivateKeySize) destroy() {
	FfiDestroyerUint64{}.Destroy(e.Size)
}

func (err WriteErrorPrivateKeySize) Error() string {
	return fmt.Sprint("PrivateKeySize",
		": ",

		"Size=",
		err.Size,
	)
}

func (self WriteErrorPrivateKeySize) Is(target error) bool {
	return target == ErrWriteErrorPrivateKeySize
}

type FfiConverterWriteError struct{}

var FfiConverterWriteErrorINSTANCE = FfiConverterWriteError{}

func (c FfiConverterWriteError) Lift(eb RustBufferI) *WriteError {
	return LiftFromRustBuffer[*WriteError](c, eb)
}

func (c FfiConverterWriteError) Lower(value *WriteError) C.RustBuffer {
	return LowerIntoRustBuffer[*WriteError](c, value)
}

func (c FfiConverterWriteError) Read(reader io.Reader) *WriteError {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &WriteError{&WriteErrorPrivateKeySize{
			Size: FfiConverterUint64INSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterWriteError.Read()", errorID))
	}
}

func (c FfiConverterWriteError) Write(writer io.Writer, value *WriteError) {
	switch variantValue := value.err.(type) {
	case *WriteErrorPrivateKeySize:
		writeInt32(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, variantValue.Size)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterWriteError.Write", value))
	}
}

type FfiDestroyerWriteError struct{}

func (_ FfiDestroyerWriteError) Destroy(value *WriteError) {
	switch variantValue := value.err.(type) {
	case WriteErrorPrivateKeySize:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerWriteError.Destroy", value))
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

type FfiConverterOptionalBytes struct{}

var FfiConverterOptionalBytesINSTANCE = FfiConverterOptionalBytes{}

func (c FfiConverterOptionalBytes) Lift(rb RustBufferI) *[]byte {
	return LiftFromRustBuffer[*[]byte](c, rb)
}

func (_ FfiConverterOptionalBytes) Read(reader io.Reader) *[]byte {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterBytesINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalBytes) Lower(value *[]byte) C.RustBuffer {
	return LowerIntoRustBuffer[*[]byte](c, value)
}

func (_ FfiConverterOptionalBytes) Write(writer io.Writer, value *[]byte) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterBytesINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalBytes struct{}

func (_ FfiDestroyerOptionalBytes) Destroy(value *[]byte) {
	if value != nil {
		FfiDestroyerBytes{}.Destroy(*value)
	}
}

type FfiConverterOptionalSubscribeItem struct{}

var FfiConverterOptionalSubscribeItemINSTANCE = FfiConverterOptionalSubscribeItem{}

func (c FfiConverterOptionalSubscribeItem) Lift(rb RustBufferI) *SubscribeItem {
	return LiftFromRustBuffer[*SubscribeItem](c, rb)
}

func (_ FfiConverterOptionalSubscribeItem) Read(reader io.Reader) *SubscribeItem {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterSubscribeItemINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalSubscribeItem) Lower(value *SubscribeItem) C.RustBuffer {
	return LowerIntoRustBuffer[*SubscribeItem](c, value)
}

func (_ FfiConverterOptionalSubscribeItem) Write(writer io.Writer, value *SubscribeItem) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterSubscribeItemINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalSubscribeItem struct{}

func (_ FfiDestroyerOptionalSubscribeItem) Destroy(value *SubscribeItem) {
	if value != nil {
		FfiDestroyerSubscribeItem{}.Destroy(*value)
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

type FfiConverterSequenceBytes struct{}

var FfiConverterSequenceBytesINSTANCE = FfiConverterSequenceBytes{}

func (c FfiConverterSequenceBytes) Lift(rb RustBufferI) [][]byte {
	return LiftFromRustBuffer[[][]byte](c, rb)
}

func (c FfiConverterSequenceBytes) Read(reader io.Reader) [][]byte {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([][]byte, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterBytesINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceBytes) Lower(value [][]byte) C.RustBuffer {
	return LowerIntoRustBuffer[[][]byte](c, value)
}

func (c FfiConverterSequenceBytes) Write(writer io.Writer, value [][]byte) {
	if len(value) > math.MaxInt32 {
		panic("[][]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterBytesINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceBytes struct{}

func (FfiDestroyerSequenceBytes) Destroy(sequence [][]byte) {
	for _, value := range sequence {
		FfiDestroyerBytes{}.Destroy(value)
	}
}

type FfiConverterSequencePublicKey struct{}

var FfiConverterSequencePublicKeyINSTANCE = FfiConverterSequencePublicKey{}

func (c FfiConverterSequencePublicKey) Lift(rb RustBufferI) []*PublicKey {
	return LiftFromRustBuffer[[]*PublicKey](c, rb)
}

func (c FfiConverterSequencePublicKey) Read(reader io.Reader) []*PublicKey {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*PublicKey, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterPublicKeyINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequencePublicKey) Lower(value []*PublicKey) C.RustBuffer {
	return LowerIntoRustBuffer[[]*PublicKey](c, value)
}

func (c FfiConverterSequencePublicKey) Write(writer io.Writer, value []*PublicKey) {
	if len(value) > math.MaxInt32 {
		panic("[]*PublicKey is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterPublicKeyINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequencePublicKey struct{}

func (FfiDestroyerSequencePublicKey) Destroy(sequence []*PublicKey) {
	for _, value := range sequence {
		FfiDestroyerPublicKey{}.Destroy(value)
	}
}

type FfiConverterSequenceEntry struct{}

var FfiConverterSequenceEntryINSTANCE = FfiConverterSequenceEntry{}

func (c FfiConverterSequenceEntry) Lift(rb RustBufferI) []Entry {
	return LiftFromRustBuffer[[]Entry](c, rb)
}

func (c FfiConverterSequenceEntry) Read(reader io.Reader) []Entry {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Entry, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterEntryINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceEntry) Lower(value []Entry) C.RustBuffer {
	return LowerIntoRustBuffer[[]Entry](c, value)
}

func (c FfiConverterSequenceEntry) Write(writer io.Writer, value []Entry) {
	if len(value) > math.MaxInt32 {
		panic("[]Entry is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterEntryINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceEntry struct{}

func (FfiDestroyerSequenceEntry) Destroy(sequence []Entry) {
	for _, value := range sequence {
		FfiDestroyerEntry{}.Destroy(value)
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

func GetManifestAndCert(data []byte) (string, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SpError](FfiConverterSpError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_func_get_manifest_and_cert(FfiConverterBytesINSTANCE.Lower(data), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func GetManifests(data []byte) (string, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SpError](FfiConverterSpError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_func_get_manifests(FfiConverterBytesINSTANCE.Lower(data), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

// Initialize logging with the default subscriber that respects RUST_LOG environment variable.
// This function is safe to call multiple times - it will only initialize logging once.
func InitLogging() {
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_streamplace_fn_func_init_logging(_uniffiStatus)
		return false
	})
}

// Initialize logging with a custom log level.
// This function is safe to call multiple times - it will only initialize logging once.
//
// # Arguments
// * `level` - Log level as a string (e.g., "trace", "debug", "info", "warn", "error")
func InitLoggingWithLevel(level string) {
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_streamplace_fn_func_init_logging_with_level(FfiConverterStringINSTANCE.Lower(level), _uniffiStatus)
		return false
	})
}

// Get this node's ticket.
func NodeIdFromTicket(ticketStr string) (*PublicKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[TicketError](FfiConverterTicketError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_func_node_id_from_ticket(FfiConverterStringINSTANCE.Lower(ticketStr), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PublicKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPublicKeyINSTANCE.Lift(_uniffiRV), nil
	}
}

func Resign(unsignedSegLabel string, unsignedSegData []byte, signedConcatData []byte, certs []byte) ([]byte, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SpError](FfiConverterSpError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_func_resign(FfiConverterStringINSTANCE.Lower(unsignedSegLabel), FfiConverterBytesINSTANCE.Lower(unsignedSegData), FfiConverterBytesINSTANCE.Lower(signedConcatData), FfiConverterBytesINSTANCE.Lower(certs), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

func Sign(manifest string, data []byte, certs []byte, gosigner GoSigner) ([]byte, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SpError](FfiConverterSpError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_func_sign(FfiConverterStringINSTANCE.Lower(manifest), FfiConverterBytesINSTANCE.Lower(data), FfiConverterBytesINSTANCE.Lower(certs), FfiConverterGoSignerINSTANCE.Lower(gosigner), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

func SignWithIngredients(manifest string, data []byte, certs []byte, ingredients [][]byte, gosigner GoSigner) ([]byte, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[SpError](FfiConverterSpError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_func_sign_with_ingredients(FfiConverterStringINSTANCE.Lower(manifest), FfiConverterBytesINSTANCE.Lower(data), FfiConverterBytesINSTANCE.Lower(certs), FfiConverterSequenceBytesINSTANCE.Lower(ingredients), FfiConverterGoSignerINSTANCE.Lower(gosigner), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), nil
	}
}

func SubscribeItemDebug(item SubscribeItem) string {
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_func_subscribe_item_debug(FfiConverterSubscribeItemINSTANCE.Lower(item), _uniffiStatus),
		}
	}))
}
