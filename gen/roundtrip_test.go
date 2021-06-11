// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package gen

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/thriftrw/protocol"
	"go.uber.org/thriftrw/protocol/stream"
	"go.uber.org/thriftrw/wire"
)

// assertRoundTrip checks if x.ToWire() results in the given Value and whether
// x.FromWire() with the given value results in the original x.
func assertRoundTrip(t *testing.T, x thriftType, v wire.Value, msg string, args ...interface{}) bool {
	message := fmt.Sprintf(msg, args...)

	if w, err := x.ToWire(); assert.NoError(t, err, "failed to serialize: %v", x) {
		if !assert.True(
			t, wire.ValuesAreEqual(v, w), "%v: %v.ToWire() != %v", message, x, v) {
			return false
		}
		// Flip v to deserialize(serialize(x.ToWire())) to ensure full round trip.
		freshV, ok := assertBinaryRoundTrip(t, w, message)
		if !assert.True(t, ok, "%v: failed encode/decode round trip for (%v.ToWire())) != %v", x, v) {
			return false
		}
		v = freshV // use the "freshest" value.
	}

	xType := reflect.TypeOf(x)
	if xType.Kind() == reflect.Ptr {
		xType = xType.Elem()
	}

	gotX := reflect.New(xType).Interface().(thriftType)
	if assert.NoError(t, gotX.FromWire(v), "FromWire: %v", message) {
		return assert.Equal(t, x, gotX, "FromWire: %v", message)
	}

	return false
}

// assertBinaryRoundTrip checks that De/Encode returns the same value.
func assertBinaryRoundTrip(t *testing.T, w wire.Value, message string) (wire.Value, bool) {
	var buff bytes.Buffer
	if !assert.NoError(t, protocol.Binary.Encode(w, &buff), "%v: failed to serialize", message) {
		return w, false
	}

	newV, err := protocol.Binary.Decode(bytes.NewReader(buff.Bytes()), w.Type())
	if !assert.NoError(t, err, "%v: failed to deserialize", message) {
		return newV, false
	}

	if !assert.True(t, wire.ValuesAreEqual(newV, w)) {
		return newV, false
	}

	return newV, true
}

// assertStreamingRoundTrip checks that the streaming Encode/Decode returns the same
// value.
//
// Note: Leverages 'ToWire' and regular binary.Encode until a 'stream.Writer'
// implementation is plumbed through.  Ideally, this should perform tests across
// the cross-product of all the wire protocol <-> type encodes/decodes.
func assertStreamingRoundTrip(t *testing.T, x thriftType, v wire.Value, msg string, args ...interface{}) bool {
	t.Helper()

	message := "streaming-" + fmt.Sprintf(msg, args...)

	// BEGIN ToWire/Binary.Encode
	var buff bytes.Buffer

	if w, err := x.ToWire(); assert.NoError(t, err, "failed to serialize: %v", x) {
		if !assert.True(
			t, wire.ValuesAreEqual(v, w), "%v: %v.ToWire() != %v", message, x, v) {
			return false
		}

		if !assert.NoError(t, protocol.Binary.Encode(w, &buff), "%v: failed to binary.Encode", message) {
			return false
		}
	}
	// END ToWire/Binary.Encode

	sp, ok := protocol.Binary.(stream.Protocol)
	require.True(t, ok)

	xType := reflect.TypeOf(x)
	if xType.Kind() == reflect.Ptr {
		xType = xType.Elem()
	}

	reader := sp.Reader(bytes.NewReader(buff.Bytes()))
	gotX, ok := reflect.New(xType).Interface().(streamingThriftType)
	require.True(t, ok)

	if assert.NoError(t, gotX.Decode(reader), "streaming decode: %v", message) {
		return assert.Equal(t, x, gotX, "streaming decode: %v", message)
	}

	return false
}
