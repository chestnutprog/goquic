package goquic

// #include <stddef.h>
// #include "adaptor.h"
import "C"
import (
	"strings"
	"unsafe"
)

//   (For QuicSpdyServerStream)
type DataStreamProcessor interface {
	ProcessData(writer QuicStream, buffer []byte) int
	OnFinRead(writer QuicStream)
}

//   (For QuicServerSession)
type DataStreamCreator interface {
	CreateIncomingDataStream(streamId uint32) DataStreamProcessor
	CreateOutgoingDataStream() DataStreamProcessor
}

type QuicStream interface {
	ProcessData(buf []byte) uint32
	OnFinRead()
	WriteHeader(header map[string][]string, is_body_empty bool)
	WriteOrBufferData(body []byte, fin bool)
}

type QuicSpdyServerStream struct {
	userStream DataStreamProcessor
	wrapper    unsafe.Pointer
	session    *QuicServerSession
}

func (writer *QuicSpdyServerStream) WriteHeader(header map[string][]string, is_body_empty bool) {
	header_c := C.initialize_map()
	for key, values := range header {
		value := strings.Join(values, ", ")
		C.insert_map(header_c, C.CString(key), C.CString(value))
	}

	if is_body_empty {
		C.quic_spdy_server_stream_write_headers(writer.wrapper, header_c, 1)
	} else {
		C.quic_spdy_server_stream_write_headers(writer.wrapper, header_c, 0)
	}
}

func (writer *QuicSpdyServerStream) WriteOrBufferData(body []byte, fin bool) {
	fin_int := C.int(0)
	if fin {
		fin_int = C.int(1)
	}

	if len(body) == 0 {
		C.quic_spdy_server_stream_write_or_buffer_data(writer.wrapper, (*C.char)(unsafe.Pointer(nil)), C.size_t(0), fin_int)
	} else {
		C.quic_spdy_server_stream_write_or_buffer_data(writer.wrapper, (*C.char)(unsafe.Pointer(&body[0])), C.size_t(len(body)), fin_int)
	}
}

func (writer *QuicSpdyServerStream) ProcessData(buf []byte) uint32 {
	return uint32(writer.userStream.ProcessData(writer, buf))
}

func (writer *QuicSpdyServerStream) OnFinRead() {
	writer.userStream.OnFinRead(writer)
}

//export CreateIncomingDataStream
func CreateIncomingDataStream(session_c unsafe.Pointer, stream_id uint32, wrapper_c unsafe.Pointer) unsafe.Pointer {
	session := (*QuicServerSession)(session_c)
	userStream := session.streamCreator.CreateIncomingDataStream(stream_id)

	stream := &QuicSpdyServerStream{
		userStream: userStream,
		session:    session,
		wrapper:    wrapper_c,
	}

	session.quicServerStreams = append(session.quicServerStreams, stream) // TODO(hodduc): cleanup

	return unsafe.Pointer(stream)
}

//export DataStreamProcessorProcessData
func DataStreamProcessorProcessData(go_data_stream_processor_c unsafe.Pointer, data unsafe.Pointer, data_len uint32, isServer int) uint32 {
	var stream QuicStream
	if isServer > 0 {
		stream = (*QuicSpdyServerStream)(go_data_stream_processor_c)
	} else {
		stream = (*QuicClientStream)(go_data_stream_processor_c)
	}
	buf := C.GoBytes(data, C.int(data_len))
	return stream.ProcessData(buf)
}

//export DataStreamProcessorOnFinRead
func DataStreamProcessorOnFinRead(go_data_stream_processor_c unsafe.Pointer, isServer int) {
	var stream QuicStream
	if isServer > 0 {
		stream = (*QuicSpdyServerStream)(go_data_stream_processor_c)
	} else {
		stream = (*QuicClientStream)(go_data_stream_processor_c)
	}
	stream.OnFinRead()
}