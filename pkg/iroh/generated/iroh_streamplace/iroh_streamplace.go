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
			return C.uniffi_iroh_streamplace_checksum_func_node_addr_from_ticket()
		})
		if checksum != 8919 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_func_node_addr_from_ticket: UniFFI API checksum mismatch")
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
			return C.uniffi_iroh_streamplace_checksum_method_socket_accept()
		})
		if checksum != 57029 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_socket_accept: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_socket_alpn()
		})
		if checksum != 42300 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_socket_alpn: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_socket_close()
		})
		if checksum != 61206 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_socket_close: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_socket_connect()
		})
		if checksum != 10698 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_socket_connect: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_socket_online()
		})
		if checksum != 7658 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_socket_online: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_socket_ticket()
		})
		if checksum != 43285 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_socket_ticket: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_stream_close()
		})
		if checksum != 42672 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_stream_close: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_stream_close_read()
		})
		if checksum != 37182 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_stream_close_read: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_stream_close_write()
		})
		if checksum != 3902 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_stream_close_write: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_stream_closed()
		})
		if checksum != 29084 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_stream_closed: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_stream_read()
		})
		if checksum != 28625 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_stream_read: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_stream_write()
		})
		if checksum != 7829 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_stream_write: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_method_stream_write_all()
		})
		if checksum != 28367 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_method_stream_write_all: UniFFI API checksum mismatch")
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
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_streamplace_checksum_constructor_socket_new()
		})
		if checksum != 4547 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh_streamplace: uniffi_iroh_streamplace_checksum_constructor_socket_new: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

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

// A wrapper for an iroh endpoint that works basically as a socket for streams.
type SocketInterface interface {
	// Accept an incoming connection and return a [`Stream`].
	Accept() (*Stream, error)
	// Get the ALPN for this socket.
	Alpn() []byte
	// Close the socket.
	Close()
	// Connect to a peer at the given [`NodeAddr`] and return a [`Stream`].
	Connect(addr *NodeAddr) (*Stream, error)
	// Wait until the socket is online.
	Online()
	// Get the ticket for this socket.
	Ticket() string
}

// A wrapper for an iroh endpoint that works basically as a socket for streams.
type Socket struct {
	ffiObject FfiObject
}

// Create a new [`Socket`] with the given [`SocketConfig`].
func NewSocket(config SocketConfig) (*Socket, error) {
	res, err := uniffiRustCallAsync[SocketNewError](
		FfiConverterSocketNewErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Socket {
			return FfiConverterSocketINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_constructor_socket_new(FfiConverterSocketConfigINSTANCE.Lower(config)),
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

// Accept an incoming connection and return a [`Stream`].
func (_self *Socket) Accept() (*Stream, error) {
	_pointer := _self.ffiObject.incrementPointer("*Socket")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[AcceptError](
		FfiConverterAcceptErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Stream {
			return FfiConverterStreamINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_socket_accept(
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

// Get the ALPN for this socket.
func (_self *Socket) Alpn() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Socket")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_socket_alpn(
				_pointer, _uniffiStatus),
		}
	}))
}

// Close the socket.
func (_self *Socket) Close() {
	_pointer := _self.ffiObject.incrementPointer("*Socket")
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
		C.uniffi_iroh_streamplace_fn_method_socket_close(
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

}

// Connect to a peer at the given [`NodeAddr`] and return a [`Stream`].
func (_self *Socket) Connect(addr *NodeAddr) (*Stream, error) {
	_pointer := _self.ffiObject.incrementPointer("*Socket")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[ConnectError](
		FfiConverterConnectErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) unsafe.Pointer {
			res := C.ffi_iroh_streamplace_rust_future_complete_pointer(handle, status)
			return res
		},
		// liftFn
		func(ffi unsafe.Pointer) *Stream {
			return FfiConverterStreamINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_socket_connect(
			_pointer, FfiConverterNodeAddrINSTANCE.Lower(addr)),
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

// Wait until the socket is online.
func (_self *Socket) Online() {
	_pointer := _self.ffiObject.incrementPointer("*Socket")
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
		C.uniffi_iroh_streamplace_fn_method_socket_online(
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

}

// Get the ticket for this socket.
func (_self *Socket) Ticket() string {
	_pointer := _self.ffiObject.incrementPointer("*Socket")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_iroh_streamplace_fn_method_socket_ticket(
				_pointer, _uniffiStatus),
		}
	}))
}
func (object *Socket) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSocket struct{}

var FfiConverterSocketINSTANCE = FfiConverterSocket{}

func (c FfiConverterSocket) Lift(pointer unsafe.Pointer) *Socket {
	result := &Socket{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_socket(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_socket(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Socket).Destroy)
	return result
}

func (c FfiConverterSocket) Read(reader io.Reader) *Socket {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSocket) Lower(value *Socket) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Socket")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterSocket) Write(writer io.Writer, value *Socket) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSocket struct{}

func (_ FfiDestroyerSocket) Destroy(value *Socket) {
	value.Destroy()
}

// A bidirectional stream over an iroh connection.
//
// In QUIC streams and connections are separate concepts. A connection can have multiple streams.
// For simplicity we expose a single bidirectional stream per connection here.
type StreamInterface interface {
	// Close the underlying connection.
	Close()
	// Close the read side of the stream.
	//
	// Note: this does not close the underlying connection.
	CloseRead() error
	// Close the write side of the stream.
	//
	// Note: this does not close the underlying connection.
	CloseWrite() error
	// Wait until the connection is closed.
	Closed()
	// Read up to n bytes from the stream.
	//
	// Due to the way uniffi works, this can't have the signature that is
	// usually used in golang code. Instead of taking a mutable buffer and
	// returning the number of bytes read, it takes the number of bytes to read
	// and returns a vector with the data read.
	//
	// Wrapping this into a more idiomatic golang interface needs a few lines
	// on the golang side.
	Read(n uint64) ([]byte, error)
	// Write up to n bytes from buf to the stream.
	Write(buf []byte) (uint32, error)
	// Write all bytes in buf to the stream.
	WriteAll(buf []byte) error
}

// A bidirectional stream over an iroh connection.
//
// In QUIC streams and connections are separate concepts. A connection can have multiple streams.
// For simplicity we expose a single bidirectional stream per connection here.
type Stream struct {
	ffiObject FfiObject
}

// Close the underlying connection.
func (_self *Stream) Close() {
	_pointer := _self.ffiObject.incrementPointer("*Stream")
	defer _self.ffiObject.decrementPointer()
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_streamplace_fn_method_stream_close(
			_pointer, _uniffiStatus)
		return false
	})
}

// Close the read side of the stream.
//
// Note: this does not close the underlying connection.
func (_self *Stream) CloseRead() error {
	_pointer := _self.ffiObject.incrementPointer("*Stream")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[ReadError](
		FfiConverterReadErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_stream_close_read(
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

// Close the write side of the stream.
//
// Note: this does not close the underlying connection.
func (_self *Stream) CloseWrite() error {
	_pointer := _self.ffiObject.incrementPointer("*Stream")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[WriteError2](
		FfiConverterWriteError2INSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_stream_close_write(
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

// Wait until the connection is closed.
func (_self *Stream) Closed() {
	_pointer := _self.ffiObject.incrementPointer("*Stream")
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
		C.uniffi_iroh_streamplace_fn_method_stream_closed(
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

}

// Read up to n bytes from the stream.
//
// Due to the way uniffi works, this can't have the signature that is
// usually used in golang code. Instead of taking a mutable buffer and
// returning the number of bytes read, it takes the number of bytes to read
// and returns a vector with the data read.
//
// Wrapping this into a more idiomatic golang interface needs a few lines
// on the golang side.
func (_self *Stream) Read(n uint64) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*Stream")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[ReadError](
		FfiConverterReadErrorINSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) RustBufferI {
			res := C.ffi_iroh_streamplace_rust_future_complete_rust_buffer(handle, status)
			return GoRustBuffer{
				inner: res,
			}
		},
		// liftFn
		func(ffi RustBufferI) []byte {
			return FfiConverterBytesINSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_stream_read(
			_pointer, FfiConverterUint64INSTANCE.Lower(n)),
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

// Write up to n bytes from buf to the stream.
func (_self *Stream) Write(buf []byte) (uint32, error) {
	_pointer := _self.ffiObject.incrementPointer("*Stream")
	defer _self.ffiObject.decrementPointer()
	res, err := uniffiRustCallAsync[WriteError2](
		FfiConverterWriteError2INSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) C.uint32_t {
			res := C.ffi_iroh_streamplace_rust_future_complete_u32(handle, status)
			return res
		},
		// liftFn
		func(ffi C.uint32_t) uint32 {
			return FfiConverterUint32INSTANCE.Lift(ffi)
		},
		C.uniffi_iroh_streamplace_fn_method_stream_write(
			_pointer, FfiConverterBytesINSTANCE.Lower(buf)),
		// pollFn
		func(handle C.uint64_t, continuation C.UniffiRustFutureContinuationCallback, data C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_poll_u32(handle, continuation, data)
		},
		// freeFn
		func(handle C.uint64_t) {
			C.ffi_iroh_streamplace_rust_future_free_u32(handle)
		},
	)

	if err == nil {
		return res, nil
	}

	return res, err
}

// Write all bytes in buf to the stream.
func (_self *Stream) WriteAll(buf []byte) error {
	_pointer := _self.ffiObject.incrementPointer("*Stream")
	defer _self.ffiObject.decrementPointer()
	_, err := uniffiRustCallAsync[WriteError2](
		FfiConverterWriteError2INSTANCE,
		// completeFn
		func(handle C.uint64_t, status *C.RustCallStatus) struct{} {
			C.ffi_iroh_streamplace_rust_future_complete_void(handle, status)
			return struct{}{}
		},
		// liftFn
		func(_ struct{}) struct{} { return struct{}{} },
		C.uniffi_iroh_streamplace_fn_method_stream_write_all(
			_pointer, FfiConverterBytesINSTANCE.Lower(buf)),
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
func (object *Stream) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterStream struct{}

var FfiConverterStreamINSTANCE = FfiConverterStream{}

func (c FfiConverterStream) Lift(pointer unsafe.Pointer) *Stream {
	result := &Stream{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_iroh_streamplace_fn_clone_stream(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_streamplace_fn_free_stream(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Stream).Destroy)
	return result
}

func (c FfiConverterStream) Read(reader io.Reader) *Stream {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterStream) Lower(value *Stream) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Stream")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterStream) Write(writer io.Writer, value *Stream) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerStream struct{}

func (_ FfiDestroyerStream) Destroy(value *Stream) {
	value.Destroy()
}

// Configuration for creating a [`Socket`].
type SocketConfig struct {
	// A 32-byte secret key for the socket.
	Secret []byte
	// The ALPN to use for this socket.
	Alpn []byte
}

func (r *SocketConfig) Destroy() {
	FfiDestroyerBytes{}.Destroy(r.Secret)
	FfiDestroyerBytes{}.Destroy(r.Alpn)
}

type FfiConverterSocketConfig struct{}

var FfiConverterSocketConfigINSTANCE = FfiConverterSocketConfig{}

func (c FfiConverterSocketConfig) Lift(rb RustBufferI) SocketConfig {
	return LiftFromRustBuffer[SocketConfig](c, rb)
}

func (c FfiConverterSocketConfig) Read(reader io.Reader) SocketConfig {
	return SocketConfig{
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
	}
}

func (c FfiConverterSocketConfig) Lower(value SocketConfig) C.RustBuffer {
	return LowerIntoRustBuffer[SocketConfig](c, value)
}

func (c FfiConverterSocketConfig) Write(writer io.Writer, value SocketConfig) {
	FfiConverterBytesINSTANCE.Write(writer, value.Secret)
	FfiConverterBytesINSTANCE.Write(writer, value.Alpn)
}

type FfiDestroyerSocketConfig struct{}

func (_ FfiDestroyerSocketConfig) Destroy(value SocketConfig) {
	value.Destroy()
}

type AcceptError struct {
	err error
}

// Convience method to turn *AcceptError into error
// Avoiding treating nil pointer as non nil error interface
func (err *AcceptError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err AcceptError) Error() string {
	return fmt.Sprintf("AcceptError: %s", err.err.Error())
}

func (err AcceptError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrAcceptErrorOther = fmt.Errorf("AcceptErrorOther")

// Variant structs
type AcceptErrorOther struct {
	message string
}

func NewAcceptErrorOther() *AcceptError {
	return &AcceptError{err: &AcceptErrorOther{}}
}

func (e AcceptErrorOther) destroy() {
}

func (err AcceptErrorOther) Error() string {
	return fmt.Sprintf("Other: %s", err.message)
}

func (self AcceptErrorOther) Is(target error) bool {
	return target == ErrAcceptErrorOther
}

type FfiConverterAcceptError struct{}

var FfiConverterAcceptErrorINSTANCE = FfiConverterAcceptError{}

func (c FfiConverterAcceptError) Lift(eb RustBufferI) *AcceptError {
	return LiftFromRustBuffer[*AcceptError](c, eb)
}

func (c FfiConverterAcceptError) Lower(value *AcceptError) C.RustBuffer {
	return LowerIntoRustBuffer[*AcceptError](c, value)
}

func (c FfiConverterAcceptError) Read(reader io.Reader) *AcceptError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &AcceptError{&AcceptErrorOther{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterAcceptError.Read()", errorID))
	}

}

func (c FfiConverterAcceptError) Write(writer io.Writer, value *AcceptError) {
	switch variantValue := value.err.(type) {
	case *AcceptErrorOther:
		writeInt32(writer, 1)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterAcceptError.Write", value))
	}
}

type FfiDestroyerAcceptError struct{}

func (_ FfiDestroyerAcceptError) Destroy(value *AcceptError) {
	switch variantValue := value.err.(type) {
	case AcceptErrorOther:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerAcceptError.Destroy", value))
	}
}

type ConnectError struct {
	err error
}

// Convience method to turn *ConnectError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ConnectError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ConnectError) Error() string {
	return fmt.Sprintf("ConnectError: %s", err.err.Error())
}

func (err ConnectError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrConnectErrorOther = fmt.Errorf("ConnectErrorOther")

// Variant structs
type ConnectErrorOther struct {
	message string
}

func NewConnectErrorOther() *ConnectError {
	return &ConnectError{err: &ConnectErrorOther{}}
}

func (e ConnectErrorOther) destroy() {
}

func (err ConnectErrorOther) Error() string {
	return fmt.Sprintf("Other: %s", err.message)
}

func (self ConnectErrorOther) Is(target error) bool {
	return target == ErrConnectErrorOther
}

type FfiConverterConnectError struct{}

var FfiConverterConnectErrorINSTANCE = FfiConverterConnectError{}

func (c FfiConverterConnectError) Lift(eb RustBufferI) *ConnectError {
	return LiftFromRustBuffer[*ConnectError](c, eb)
}

func (c FfiConverterConnectError) Lower(value *ConnectError) C.RustBuffer {
	return LowerIntoRustBuffer[*ConnectError](c, value)
}

func (c FfiConverterConnectError) Read(reader io.Reader) *ConnectError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &ConnectError{&ConnectErrorOther{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterConnectError.Read()", errorID))
	}

}

func (c FfiConverterConnectError) Write(writer io.Writer, value *ConnectError) {
	switch variantValue := value.err.(type) {
	case *ConnectErrorOther:
		writeInt32(writer, 1)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterConnectError.Write", value))
	}
}

type FfiDestroyerConnectError struct{}

func (_ FfiDestroyerConnectError) Destroy(value *ConnectError) {
	switch variantValue := value.err.(type) {
	case ConnectErrorOther:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerConnectError.Destroy", value))
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

type ReadError struct {
	err error
}

// Convience method to turn *ReadError into error
// Avoiding treating nil pointer as non nil error interface
func (err *ReadError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err ReadError) Error() string {
	return fmt.Sprintf("ReadError: %s", err.err.Error())
}

func (err ReadError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrReadErrorOther = fmt.Errorf("ReadErrorOther")

// Variant structs
type ReadErrorOther struct {
	message string
}

func NewReadErrorOther() *ReadError {
	return &ReadError{err: &ReadErrorOther{}}
}

func (e ReadErrorOther) destroy() {
}

func (err ReadErrorOther) Error() string {
	return fmt.Sprintf("Other: %s", err.message)
}

func (self ReadErrorOther) Is(target error) bool {
	return target == ErrReadErrorOther
}

type FfiConverterReadError struct{}

var FfiConverterReadErrorINSTANCE = FfiConverterReadError{}

func (c FfiConverterReadError) Lift(eb RustBufferI) *ReadError {
	return LiftFromRustBuffer[*ReadError](c, eb)
}

func (c FfiConverterReadError) Lower(value *ReadError) C.RustBuffer {
	return LowerIntoRustBuffer[*ReadError](c, value)
}

func (c FfiConverterReadError) Read(reader io.Reader) *ReadError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &ReadError{&ReadErrorOther{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterReadError.Read()", errorID))
	}

}

func (c FfiConverterReadError) Write(writer io.Writer, value *ReadError) {
	switch variantValue := value.err.(type) {
	case *ReadErrorOther:
		writeInt32(writer, 1)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterReadError.Write", value))
	}
}

type FfiDestroyerReadError struct{}

func (_ FfiDestroyerReadError) Destroy(value *ReadError) {
	switch variantValue := value.err.(type) {
	case ReadErrorOther:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerReadError.Destroy", value))
	}
}

type SocketNewError struct {
	err error
}

// Convience method to turn *SocketNewError into error
// Avoiding treating nil pointer as non nil error interface
func (err *SocketNewError) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err SocketNewError) Error() string {
	return fmt.Sprintf("SocketNewError: %s", err.err.Error())
}

func (err SocketNewError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrSocketNewErrorOther = fmt.Errorf("SocketNewErrorOther")

// Variant structs
type SocketNewErrorOther struct {
	message string
}

func NewSocketNewErrorOther() *SocketNewError {
	return &SocketNewError{err: &SocketNewErrorOther{}}
}

func (e SocketNewErrorOther) destroy() {
}

func (err SocketNewErrorOther) Error() string {
	return fmt.Sprintf("Other: %s", err.message)
}

func (self SocketNewErrorOther) Is(target error) bool {
	return target == ErrSocketNewErrorOther
}

type FfiConverterSocketNewError struct{}

var FfiConverterSocketNewErrorINSTANCE = FfiConverterSocketNewError{}

func (c FfiConverterSocketNewError) Lift(eb RustBufferI) *SocketNewError {
	return LiftFromRustBuffer[*SocketNewError](c, eb)
}

func (c FfiConverterSocketNewError) Lower(value *SocketNewError) C.RustBuffer {
	return LowerIntoRustBuffer[*SocketNewError](c, value)
}

func (c FfiConverterSocketNewError) Read(reader io.Reader) *SocketNewError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &SocketNewError{&SocketNewErrorOther{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterSocketNewError.Read()", errorID))
	}

}

func (c FfiConverterSocketNewError) Write(writer io.Writer, value *SocketNewError) {
	switch variantValue := value.err.(type) {
	case *SocketNewErrorOther:
		writeInt32(writer, 1)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterSocketNewError.Write", value))
	}
}

type FfiDestroyerSocketNewError struct{}

func (_ FfiDestroyerSocketNewError) Destroy(value *SocketNewError) {
	switch variantValue := value.err.(type) {
	case SocketNewErrorOther:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerSocketNewError.Destroy", value))
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

type WriteError2 struct {
	err error
}

// Convience method to turn *WriteError2 into error
// Avoiding treating nil pointer as non nil error interface
func (err *WriteError2) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err WriteError2) Error() string {
	return fmt.Sprintf("WriteError2: %s", err.err.Error())
}

func (err WriteError2) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrWriteError2Other = fmt.Errorf("WriteError2Other")

// Variant structs
type WriteError2Other struct {
	message string
}

func NewWriteError2Other() *WriteError2 {
	return &WriteError2{err: &WriteError2Other{}}
}

func (e WriteError2Other) destroy() {
}

func (err WriteError2Other) Error() string {
	return fmt.Sprintf("Other: %s", err.message)
}

func (self WriteError2Other) Is(target error) bool {
	return target == ErrWriteError2Other
}

type FfiConverterWriteError2 struct{}

var FfiConverterWriteError2INSTANCE = FfiConverterWriteError2{}

func (c FfiConverterWriteError2) Lift(eb RustBufferI) *WriteError2 {
	return LiftFromRustBuffer[*WriteError2](c, eb)
}

func (c FfiConverterWriteError2) Lower(value *WriteError2) C.RustBuffer {
	return LowerIntoRustBuffer[*WriteError2](c, value)
}

func (c FfiConverterWriteError2) Read(reader io.Reader) *WriteError2 {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &WriteError2{&WriteError2Other{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterWriteError2.Read()", errorID))
	}

}

func (c FfiConverterWriteError2) Write(writer io.Writer, value *WriteError2) {
	switch variantValue := value.err.(type) {
	case *WriteError2Other:
		writeInt32(writer, 1)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterWriteError2.Write", value))
	}
}

type FfiDestroyerWriteError2 struct{}

func (_ FfiDestroyerWriteError2) Destroy(value *WriteError2) {
	switch variantValue := value.err.(type) {
	case WriteError2Other:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerWriteError2.Destroy", value))
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

func NodeAddrFromTicket(ticketStr string) (*NodeAddr, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[TicketError](FfiConverterTicketError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_streamplace_fn_func_node_addr_from_ticket(FfiConverterStringINSTANCE.Lower(ticketStr), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *NodeAddr
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterNodeAddrINSTANCE.Lift(_uniffiRV), nil
	}
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
