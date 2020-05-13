// Package errors wrapper grpc error for convenient usage
package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	epb "google.golang.org/genproto/googleapis/rpc/errdetails"
	spb "google.golang.org/genproto/googleapis/rpc/status"
)

// Parse ...
func Parse(err error) (*status.Status, bool) {
	s, ok := status.FromError(err)
	if !ok {
		// try to unmarshal
		var sp spb.Status
		nerr := json.Unmarshal([]byte(err.Error()), &sp)
		if nerr != nil {
			return nil, false
		}

		return status.FromProto(&sp), true
	}

	return s, ok
}

// StatusMessage ...
func StatusMessage(err error) string {
	s, ok := status.FromError(err)
	if !ok {
		return err.Error()
	}

	return s.Message()
}

// Marshal ...
func Marshal(err error) error {
	s, ok := status.FromError(err)
	if !ok {
		return err
	}
	bs, _ := json.Marshal(s.Proto())
	return errors.New(string(bs))
}

// BuiltIn if it's built-in support, it means now run under
// grpc framework, and grpc will auto unmarshal status
// if not, we have to unmarshal and marshal manual
func BuiltIn(flag bool) {
	DefaultErr.BuiltIn(flag)
}

// Error ...
type Error struct {
	errCode    int32
	builtIn    bool
	escapePool bool
}

// From errors code increment from the given code
func From(code int32) *Error {
	return &Error{
		errCode: code,
		builtIn: true,
	}
}

// error pool
var (
	DefaultErr = From(0)
	errPool    = map[int32]int32{}
)

// BuiltIn ...
func (er *Error) BuiltIn(flag bool) *Error {
	er.builtIn = flag
	return er
}

// EscapePool ...
func (er *Error) EscapePool(flag bool) *Error {
	er.escapePool = flag
	return er
}

func (er *Error) addError(code interface{}, msg string) error {
	c := AssertCode(code)
	c += er.errCode
	if !er.escapePool {
		e, ok := errPool[c]
		if ok {
			panic(fmt.Sprintf("duplate error: %d; code: %d", e, code))
		}

		errPool[c] = c
	}

	s := status.New(codes.Code(c), msg)
	return er.marshal(s)
}

func (er *Error) marshal(s *status.Status) error {
	if er.builtIn {
		return s.Err()
	}

	bs, _ := json.Marshal(s.Proto())
	return errors.New(string(bs))
}

// Add ...
func (er *Error) Add(code interface{}, msg string) error {
	return er.addError(code, msg)
}

// Internal ...
func (er *Error) Internal(code interface{}) func(string, error) error {
	return func(msg string, err error) error {
		s := status.New(codes.Code(AssertCode(code)), msg)
		if err != nil {
			s, _ = s.WithDetails(&epb.DebugInfo{
				Detail: string(err.Error()),
			})
		}

		return er.marshal(s)
	}
}

// New errors begin from 0
func New(code interface{}, msg string) error {
	return DefaultErr.addError(code, msg)
}

// Internal only pass debuginfo while internal error happens
func Internal(code interface{}) func(string, error) error {
	return DefaultErr.Internal(code)
}

// WithDetails ...
// TODO change details to key value
func WithDetails(err error, details ...string) error {
	s, ok := status.FromError(err)
	if !ok {
		// try to unmarshal
		var sp spb.Status
		nerr := json.Unmarshal([]byte(err.Error()), &sp)
		if nerr != nil {
			return nil
		}

		s = status.FromProto(&sp)
	}
	for _, detail := range details {
		s, _ = s.WithDetails(&epb.DebugInfo{
			Detail: detail,
		})
	}

	return s.Err()
}

// AssertCode check if the underlying type of code is int32
func AssertCode(code interface{}) int32 {
	codeType := reflect.TypeOf(code)

	if codeType.Kind() != reflect.Int32 && codeType.Kind() != reflect.Int {
		panic("code should be kind of int32")
	}

	codeValue := reflect.ValueOf(code)
	return int32(codeValue.Int())
}
