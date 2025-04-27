package kratos

/*
   #cgo LDFLAGS: -L./libs -lcommon -lm -ldl -lpthread
   #include <stdlib.h>

   typedef struct {
       const char* time;
       const char* status;
       const char* module;
       const char* source;
       const char* message;
   } CLog;

   typedef struct {
       const char* from;
       const char* to;
       const char* service;
       const char* status;
       const char* module;
       const char* source;
       const char* message;
   } CListLogReq;

   extern int query_logs_ffi(const char* log_dir, const CListLogReq* query, CLog** results, int* len);
   extern void free_logs(CLog* results, int len);
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type Log struct {
	Time    string
	Status  string
	Module  string
	Source  string
	Message string
}

type QueryParams struct {
	From    string
	To      string
	Service string
	Status  string
	Module  string
	Source  string
	Message string
}

func RsQueryLogs(logDir string, params *QueryParams) ([]Log, error) {
	cQuery := C.CListLogReq{
		from:    strToC(params.From),
		to:      strToC(params.To),
		service: strToC(params.Service),
		status:  strToC(params.Status),
		module:  strToC(params.Module),
		source:  strToC(params.Source),
		message: strToC(params.Message),
	}
	defer freeCQuery(&cQuery)

	var (
		cResults *C.CLog
		cLen     C.int
	)
	ret := C.query_logs_ffi(
		C.CString(logDir),
		&cQuery,
		&cResults,
		&cLen,
	)
	defer C.free(unsafe.Pointer(C.CString(logDir)))

	if ret != 0 {
		return nil, errorCodeToError(ret)
	}

	return convertResults(cResults, int(cLen)), nil
}

func strToC(s string) *C.char {
	if s == "" {
		return nil
	}
	cStr := C.CString(s)
	return cStr
}

func freeCQuery(q *C.CListLogReq) {
	if q.from != nil {
		C.free(unsafe.Pointer(q.from))
	}
	if q.to != nil {
		C.free(unsafe.Pointer(q.to))
	}
	if q.service != nil {
		C.free(unsafe.Pointer(q.service))
	}
	if q.status != nil {
		C.free(unsafe.Pointer(q.status))
	}
	if q.module != nil {
		C.free(unsafe.Pointer(q.module))
	}
	if q.source != nil {
		C.free(unsafe.Pointer(q.source))
	}
	if q.message != nil {
		C.free(unsafe.Pointer(q.message))
	}
}

func errorCodeToError(code C.int) error {
	switch code {
	case 0:
		return nil
	case -1:
		return fmt.Errorf("invalid arguments")
	case -2:
		return fmt.Errorf("string conversion failed")
	case -3:
		return fmt.Errorf("log directory not found")
	case -4:
		return fmt.Errorf("file I/O error")
	case -255:
		return fmt.Errorf("internal rust panic")
	default:
		return fmt.Errorf("unknown error (code %d)", code)
	}
}

func convertResults(cResults *C.CLog, length int) []Log {
	if length == 0 || cResults == nil {
		return nil
	}

	results := make([]Log, length)
	slice := (*[1 << 30]C.CLog)(unsafe.Pointer(cResults))[:length:length]

	for i, cLog := range slice {
		results[i] = Log{
			Time:    C.GoString(cLog.time),
			Status:  C.GoString(cLog.status),
			Module:  C.GoString(cLog.module),
			Source:  C.GoString(cLog.source),
			Message: C.GoString(cLog.message),
		}
	}

	C.free_logs(cResults, C.int(length))

	return results
}
