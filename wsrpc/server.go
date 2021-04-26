// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package rpc provides access to the exported methods of an object across a
	network or other I/O connection.  A server registers an object, making it visible
	as a service with the name of the type of the object.  After registration, exported
	methods of the object will be accessible remotely.  A server may register multiple
	objects (services) of different types but it is an error to register multiple
	objects of the same type.

	Only methods that satisfy these criteria will be made available for remote access;
	other methods will be ignored:

		- the method's type is exported.
		- the method is exported.
		- the method has two arguments, both exported (or builtin) types.
		- the method's second argument is a pointer.
		- the method has return type error.

	In effect, the method must look schematically like

		func (t *T) MethodName(argType T1, replyType *T2) error

	where T1 and T2 can be marshaled by encoding/gob.
	These requirements apply even if a different codec is used.
	(In the future, these requirements may soften for custom codecs.)

	The method's first argument represents the arguments provided by the caller; the
	second argument represents the result parameters to be returned to the caller.
	The method's return value, if non-nil, is passed back as a string that the client
	sees as if created by errors.New.  If an error is returned, the reply parameter
	will not be sent back to the client.

	The server may handle requests on a single connection by calling ServeConn.  More
	typically it will create a network listener and call Accept or, for an HTTP
	listener, HandleHTTP and http.Serve.

	A client wishing to use the service establishes a connection and then invokes
	NewClient on the connection.  The convenience function Dial (DialHTTP) performs
	both steps for a raw network connection (an HTTP connection).  The resulting
	Client object has two methods, Call and Go, that specify the service and method to
	call, a pointer containing the arguments, and a pointer to receive the result
	parameters.

	The Call method waits for the remote call to complete while the Go method
	launches the call asynchronously and signals completion using the Call
	structure's Done channel.

	Unless an explicit codec is set up, package encoding/gob is used to
	transport the data.

	Here is a simple example.  A server wishes to export an object of type Arith:

		package server

		import "errors"

		type Args struct {
			A, B int
		}

		type Quotient struct {
			Quo, Rem int
		}

		type Arith int

		func (t *Arith) Multiply(args *Args, reply *int) error {
			*reply = args.A * args.B
			return nil
		}

		func (t *Arith) Divide(args *Args, quo *Quotient) error {
			if args.B == 0 {
				return errors.New("divide by zero")
			}
			quo.Quo = args.A / args.B
			quo.Rem = args.A % args.B
			return nil
		}

	The server calls (for HTTP service):

		arith := new(Arith)
		rpc.Register(arith)
		rpc.HandleHTTP()
		l, e := net.Listen("tcp", ":1234")
		if e != nil {
			log.Fatal("listen error:", e)
		}
		go http.Serve(l, nil)

	At this point, clients can see a service "Arith" with methods "Arith.Multiply" and
	"Arith.Divide".  To invoke one, a client first dials the server:

		client, err := rpc.DialHTTP("tcp", serverAddress + ":1234")
		if err != nil {
			log.Fatal("dialing:", err)
		}

	Then it can make a remote call:

		// Synchronous call
		args := &server.Args{7,8}
		var reply int
		err = client.Call("Arith.Multiply", args, &reply)
		if err != nil {
			log.Fatal("arith error:", err)
		}
		fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)

	or

		// Asynchronous call
		quotient := new(Quotient)
		divCall := client.Go("Arith.Divide", args, quotient, nil)
		replyCall := <-divCall.Done	// will be equal to divCall
		// check errors, print, etc.

	A server implementation will often provide a simple, type-safe wrapper for the
	client.

	The net/rpc package is frozen and is not accepting new features.
*/
package wsrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

// Defaults used by HandleHTTP
const (
	DefaultRPCPath   = "/rpc"
	DefaultDebugPath = "/debug/rpc"
)

// Precompute the reflect type for error. Can't use error directly
// because Typeof takes an empty interface value. This is annoying.
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()
var typeOfConn = reflect.TypeOf(&Conn{})

type methodType struct {
	ArgType    reflect.Type
	ReplyType  reflect.Type
	method     reflect.Method
	numCalls   uint
	sync.Mutex // protects counters
}

type service struct {
	method map[string]*methodType // registered methods
	rcvr   reflect.Value          // receiver of methods for the service
	typ    reflect.Type           // type of the receiver
	name   string                 // name of service
}

// Args for Call
type Args struct {
	Arg    reflect.Value
	Reply  reflect.Value
	Method string
	mType  *methodType
	RawReq json.RawMessage
}

// Request is a header written before every RPC call. It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Request struct {
	next          *Request // for free list in Server
	ServiceMethod string   // format: "Service.Method"
	Seq           uint64   // sequence number chosen by client
}

// Response is a header written before every RPC return. It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.
type Response struct {
	next          *Response // for free list in Server
	ServiceMethod string    // echoes that of the Request
	Error         string    // error, if any.
	Seq           uint64    // echoes that of the request
}

// InitHandler ...
type InitHandler func(*Conn)

// ServiceHandler ...
type ServiceHandler func(*Conn, *Args) (interface{}, error)

// WrapHandler ...
type WrapHandler func(ServiceHandler) ServiceHandler

// Server represents an RPC Server.
type Server struct {
	onMissingMethod MissingMethodFunc
	onWrap          WrapHandler
	serviceMap      map[string]*service
	freeReq         *Request
	freeResp        *Response

	mu       sync.RWMutex // protects the serviceMap
	reqLock  sync.Mutex   // protects freeReq
	respLock sync.Mutex   // protects freeResp
}

// MissingMethodFunc conn, method, params
type MissingMethodFunc func(*Conn, string, json.RawMessage) (interface{}, error)

// ErrMissingServiceMethod error message
type ErrMissingServiceMethod struct {
	msg string
}

func (err ErrMissingServiceMethod) Error() string {
	return err.msg
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{serviceMap: make(map[string]*service)}
}

// DefaultServer is the default instance of *Server.
var DefaultServer = NewServer()

// Is this an exported - upper case - name?
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// OnMissingMethod ...
func (server *Server) OnMissingMethod(handler MissingMethodFunc) {
	server.onMissingMethod = handler
}

// OnWrap ...
func (server *Server) OnWrap(handler WrapHandler) {
	server.onWrap = handler
}

// Is this type exported or a builtin?
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// Is this type exported or a builtin?
func isConnType(t reflect.Type) bool {
	return t == typeOfConn
}

// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- two arguments, both of exported type
//	- the second argument is a pointer
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.
func (server *Server) Register(rcvr interface{}) error {
	return server.register(rcvr, "", false)
}

// RegisterName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.
func (server *Server) RegisterName(name string, rcvr interface{}) error {
	return server.register(rcvr, name, true)
}

func (server *Server) register(rcvr interface{}, name string, useName bool) error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.serviceMap == nil {
		server.serviceMap = make(map[string]*service)
	}
	s := new(service)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	sname := reflect.Indirect(s.rcvr).Type().Name()
	if useName {
		sname = name
	}
	if sname == "" {
		s := "rpc.Register: no service name for type " + s.typ.String()
		return errors.New(s)
	}
	if !isExported(sname) && !useName {
		s := "rpc.Register: type " + sname + " is not exported"
		return errors.New(s)
	}
	if _, present := server.serviceMap[sname]; present {
		return errors.New("rpc: service already defined: " + sname)
	}
	s.name = sname

	// Install the methods
	s.method = suitableMethods(s.typ, true)

	if len(s.method) == 0 {
		str := ""

		// To help the user, see if a pointer receiver would work.
		method := suitableMethods(reflect.PtrTo(s.typ), false)
		if len(method) != 0 {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type"
		}
		return errors.New(str)
	}
	server.serviceMap[s.name] = s
	return nil
}

// suitableMethods returns suitable Rpc methods of typ, it will report
// error using log if reportErr is true.
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		if mtype.NumIn() != 4 {
			if reportErr {
				fmt.Println("method", mname, "has wrong number of ins:", mtype.NumIn())
			}
			continue
		}
		// Second arg need not be a pointer.
		argType := mtype.In(1)
		if !isConnType(argType) {
			if reportErr {
				fmt.Println(mname, "first argument is not Conn")
			}
			continue
		}
		// Second arg need not be a pointer.
		argType = mtype.In(2)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				fmt.Println(mname, "argument type not exported:", argType)
			}
			continue
		}
		// Third arg must be a pointer.
		replyType := mtype.In(3)
		if replyType.Kind() != reflect.Ptr {
			if reportErr {
				fmt.Println("method", mname, "reply type not a pointer:", replyType)
			}
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			if reportErr {
				fmt.Println("method", mname, "reply type not exported:", replyType)
			}
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {
			if reportErr {
				fmt.Println("method", mname, "has wrong number of outs:", mtype.NumOut())
			}
			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			if reportErr {
				fmt.Println("method", mname, "returns", returnType.String(), "not error")
			}
			continue
		}
		methods[mname] = &methodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

// A value sent as a placeholder for the server's response value when the server
// receives an invalid request. It is never decoded by the client since the Response
// contains an error when it is used.
var invalidRequest = struct{}{}

func (server *Server) sendResponse(sending *sync.Mutex, req *Request, reply interface{}, codec ServerCodec, errMsg error) {
	resp := server.getResponse()
	// Encode the response header
	resp.ServiceMethod = req.ServiceMethod
	if errMsg != nil {
		resp.Error = errMsg.Error()
		reply = invalidRequest
	}
	resp.Seq = req.Seq
	sending.Lock()
	err := codec.WriteResponse(resp, reply)
	if debugLog && err != nil {
		fmt.Println("rpc: writing response:", err)
	}
	sending.Unlock()
	server.freeResponse(resp)
}

func (m *methodType) NumCalls() (n uint) {
	m.Lock()
	n = m.numCalls
	m.Unlock()
	return n
}

func (s *service) call(conn *Conn, args *Args) (resp interface{}, err error) {
	args.mType.Lock()
	args.mType.numCalls++
	args.mType.Unlock()
	function := args.mType.method.Func
	// Invoke the method, providing a new value for the reply.
	returnValues := function.Call([]reflect.Value{s.rcvr,
		reflect.ValueOf(conn), args.Arg, args.Reply})
	// The return value for the method is an error.
	errInter := returnValues[0].Interface()
	if errInter != nil {
		err = errInter.(error)
	}

	resp = args.Reply.Interface()
	return
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.
func (server *Server) ServeCodec(req *http.Request, codec ServerCodec, onInit ...InitHandler) {
	sending := new(sync.Mutex)
	conn := NewConn(req, sending, codec)

	for _, fn := range onInit {
		fn(conn)
	}

	for {
		service, req, args, keepReading, err := server.readRequest(codec)
		if debugLog && err != nil && err != io.EOF {
			fmt.Println(err)
		}
		if !keepReading {
			break
		}

		args.RawReq = codec.GetParams()
		args.Method = codec.GetMethod()

		switch err.(type) {
		case ErrMissingServiceMethod:
			if server.onMissingMethod != nil {
				go func() {
					reply, err := server.onMissingMethod(conn, args.Method, args.RawReq)
					server.sendResponse(sending, req, reply, codec, err)
					server.freeRequest(req)
				}()
				continue
			}
		case nil:
			go func() {
				var (
					reply interface{}
					err   error
				)

				if server.onWrap != nil {
					reply, err = server.onWrap(service.call)(conn, args)
				} else {
					reply, err = service.call(conn, args)
				}

				server.sendResponse(sending, req, reply, codec, err)
				server.freeRequest(req)
			}()
			continue
		}

		// send a response if we actually managed to read a header.
		if req != nil {
			server.sendResponse(sending, req, invalidRequest, codec, err)
			server.freeRequest(req)
		}

	}

	conn.ternimating()

	//  close may write in that conn, just prevnet that
	sending.Lock()
	codec.Close()
	sending.Unlock()
}

func (server *Server) getRequest() *Request {
	server.reqLock.Lock()
	req := server.freeReq
	if req == nil {
		req = new(Request)
	} else {
		server.freeReq = req.next
		*req = Request{}
	}
	server.reqLock.Unlock()
	return req
}

func (server *Server) freeRequest(req *Request) {
	server.reqLock.Lock()
	req.next = server.freeReq
	server.freeReq = req
	server.reqLock.Unlock()
}

func (server *Server) getResponse() *Response {
	server.respLock.Lock()
	resp := server.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		server.freeResp = resp.next
		*resp = Response{}
	}
	server.respLock.Unlock()
	return resp
}

func (server *Server) freeResponse(resp *Response) {
	server.respLock.Lock()
	resp.next = server.freeResp
	server.freeResp = resp
	server.respLock.Unlock()
}

func (server *Server) readRequest(codec ServerCodec,
) (service *service, req *Request, args *Args, keepReading bool, err error) {
	args = &Args{}
	service, req, args.mType, keepReading, err = server.readRequestHeader(codec)
	if err != nil {
		if !keepReading {
			return
		}
		// discard body
		readErr := codec.ReadRequestBody(nil)
		if readErr != nil {
			err = readErr
		}
		return
	}

	// Decode the argument value.
	argIsValue := false // if true, need to indirect before calling.
	if args.mType.ArgType.Kind() == reflect.Ptr {
		args.Arg = reflect.New(args.mType.ArgType.Elem())
	} else {
		args.Arg = reflect.New(args.mType.ArgType)
		argIsValue = true
	}

	// argv guaranteed to be a pointer now.
	if err = codec.ReadRequestBody(args.Arg.Interface()); err != nil {
		return
	}
	if argIsValue {
		args.Arg = args.Arg.Elem()
	}

	args.Reply = reflect.New(args.mType.ReplyType.Elem())
	return
}

func (server *Server) readRequestHeader(codec ServerCodec,
) (service *service, req *Request, mtype *methodType, keepReading bool, err error) {
	// Grab the request header.
	req = server.getRequest()
	err = codec.ReadRequestHeader(req)
	if err != nil {
		req = nil
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.New("rpc: server cannot decode request: " + err.Error())
		return
	}

	// We read the header successfully. If we see an error now,
	// we can still recover and move on to the next request.
	keepReading = true

	dot := strings.LastIndex(req.ServiceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc: service/method request ill-formed: " + req.ServiceMethod)
		return
	}
	serviceName := req.ServiceMethod[:dot]
	methodName := req.ServiceMethod[dot+1:]

	// Look up the request.
	server.mu.RLock()
	service = server.serviceMap[serviceName]
	server.mu.RUnlock()
	if service == nil {
		err = ErrMissingServiceMethod{"rpc: can't find service " + req.ServiceMethod}
		return
	}
	mtype = service.method[methodName]
	if mtype == nil {
		err = ErrMissingServiceMethod{"rpc: can't find service " + req.ServiceMethod}
	}
	return
}

// OnConnect ...
func (server *Server) OnConnect(r *http.Request, ws *websocket.Conn, onInit ...InitHandler) {
	rwc := NewReadWriteCloser(ws)
	codec := NewServerCodec(rwc)
	server.ServeCodec(r, codec, onInit...)
}

// A ServerCodec implements reading of RPC requests and writing of
// RPC responses for the server side of an RPC conn.
// The server calls ReadRequestHeader and ReadRequestBody in pairs
// to read requests from the connection, and it calls WriteResponse to
// write a response back. The server calls Close when finished with the
// connection. ReadRequestBody may be called with a nil
// argument to force the body of the request to be read and discarded.
type ServerCodec interface {
	ReadRequestHeader(*Request) error
	ReadRequestBody(interface{}) error
	// WriteResponse must be safe for concurrent use by multiple goroutines.
	WriteResponse(*Response, interface{}) error
	WriteNotification(string, interface{}) error
	WriteNotificationEx(string, interface{}) error
	// Addtion params
	GetParams() (parama json.RawMessage)
	// Addtion method
	GetMethod() (method string)

	Close() error
}
