// Automatically generated by MockGen. DO NOT EDIT!
// Source: io (interfaces: Writer,WriteCloser)

package frame

import (
	gomock "github.com/golang/mock/gomock"
)

// Mock of Writer interface
type MockWriter struct {
	ctrl     *gomock.Controller
	recorder *_MockWriterRecorder
}

// Recorder for MockWriter (not exported)
type _MockWriterRecorder struct {
	mock *MockWriter
}

func NewMockWriter(ctrl *gomock.Controller) *MockWriter {
	mock := &MockWriter{ctrl: ctrl}
	mock.recorder = &_MockWriterRecorder{mock}
	return mock
}

func (_m *MockWriter) EXPECT() *_MockWriterRecorder {
	return _m.recorder
}

func (_m *MockWriter) Write(_param0 []byte) (int, error) {
	ret := _m.ctrl.Call(_m, "Write", _param0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockWriterRecorder) Write(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Write", arg0)
}

// Mock of WriteCloser interface
type MockWriteCloser struct {
	ctrl     *gomock.Controller
	recorder *_MockWriteCloserRecorder
}

// Recorder for MockWriteCloser (not exported)
type _MockWriteCloserRecorder struct {
	mock *MockWriteCloser
}

func NewMockWriteCloser(ctrl *gomock.Controller) *MockWriteCloser {
	mock := &MockWriteCloser{ctrl: ctrl}
	mock.recorder = &_MockWriteCloserRecorder{mock}
	return mock
}

func (_m *MockWriteCloser) EXPECT() *_MockWriteCloserRecorder {
	return _m.recorder
}

func (_m *MockWriteCloser) Close() error {
	ret := _m.ctrl.Call(_m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockWriteCloserRecorder) Close() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Close")
}

func (_m *MockWriteCloser) Write(_param0 []byte) (int, error) {
	ret := _m.ctrl.Call(_m, "Write", _param0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockWriteCloserRecorder) Write(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Write", arg0)
}
