package httpthrift

import (
	"net/http"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/dt/go-metrics-reporting"
)

type HasProcessFunc interface {
	GetProcessorFunction(key string) (processor thrift.TProcessorFunction, ok bool)
}
type ThriftOverHTTPHandler struct {
	HasProcessFunc
}

func NewThriftOverHTTPHandler(p HasProcessFunc) *ThriftOverHTTPHandler {
	return &ThriftOverHTTPHandler{p}
}

// borrowed from generated thrift code, but with instrumentation added.
func (p ThriftOverHTTPHandler) Process(iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {
	name, _, seqId, err := iprot.ReadMessageBegin()
	if err != nil {
		return false, err
	}

	if processor, ok := p.GetProcessorFunction(name); ok {
		start := time.Now()
		success, err = processor.Process(seqId, iprot, oprot)
		report.TimeSince(name, start)
		return
	}

	iprot.Skip(thrift.STRUCT)
	iprot.ReadMessageEnd()
	e := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, "Unknown function "+name)

	oprot.WriteMessageBegin(name, thrift.EXCEPTION, seqId)
	e.Write(oprot)
	oprot.WriteMessageEnd()
	oprot.Flush()

	return false, e
}

func (h ThriftOverHTTPHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	if req.Method == "POST" {
		var in *thrift.TMemoryBuffer
		size := int(req.ContentLength)
		if size > 0 {
			in = thrift.NewTMemoryBufferLen(size)
		} else {
			in = thrift.NewTMemoryBuffer()
		}

		in.ReadFrom(req.Body)
		defer req.Body.Close()

		iprot := thrift.NewTBinaryProtocol(in, true, true)

		outbuf := thrift.NewTMemoryBuffer()
		oprot := thrift.NewTBinaryProtocol(outbuf, true, true)

		ok, err := h.Process(iprot, oprot)

		if ok {
			outbuf.WriteTo(out)
		} else {
			http.Error(out, err.Error(), 500)
		}
	} else {
		http.Error(out, "Must POST TBinary encoded thrift RPC", 401)
	}
}