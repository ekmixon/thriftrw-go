// Copyright (c) 2021 Uber Technologies, Inc.
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

package binary

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"go.uber.org/thriftrw/internal/iface"
	"go.uber.org/thriftrw/protocol/stream"
	"go.uber.org/thriftrw/wire"
)

// Default is the default implementation of the Thrift Binary Protocol.
var Default = new(Protocol)

// Protocol implements the Thrift Binary Protocol.
type Protocol struct {
	iface.Private
}

var _ stream.Protocol = (*Protocol)(nil)
var _ stream.Handler = (*Protocol)(nil)

// Encode the given Value and write the result to the given Writer.
func (*Protocol) Encode(v wire.Value, w io.Writer) error {
	writer := BorrowWriter(w)
	err := writer.WriteValue(v)
	ReturnWriter(writer)
	return err
}

// Decode reads a Value of the given type from the given Reader.
func (*Protocol) Decode(r io.ReaderAt, t wire.Type) (wire.Value, error) {
	reader := NewReader(r)
	value, _, err := reader.ReadValue(t, 0)
	return value, err
}

// Writer builds a stream writer that writes to the provided stream using the
// Thrift Binary Protocol.
func (*Protocol) Writer(w io.Writer) stream.Writer {
	return NewStreamWriter(w)
}

// Reader builds a stream reader that reads from the provided stream using the
// Thrift Binary Protocol.
func (*Protocol) Reader(r io.Reader) stream.Reader {
	return NewStreamReader(r)
}

// EncodeEnveloped encodes the enveloped value and writes the result
// to the given Writer.
func (*Protocol) EncodeEnveloped(e wire.Envelope, w io.Writer) error {
	writer := BorrowWriter(w)
	err := writer.WriteEnveloped(e)
	ReturnWriter(writer)
	return err
}

// DecodeEnveloped reads an enveloped value from the given Reader.
// Enveloped values are assumed to be TStructs.
func (*Protocol) DecodeEnveloped(r io.ReaderAt) (wire.Envelope, error) {
	reader := NewReader(r)
	e, err := reader.ReadEnveloped()
	return e, err
}

// DecodeRequest specializes Decode and replaces DecodeEnveloped for the
// specific purpose of decoding request structs that may or may not have an
// envelope.
// This allows a Thrift request handler to transparently accept requests
// regardless of whether the caller submits an envelope.
// The caller specifies the expected envelope type, one of OneWay or Unary, on
// which the decoder asserts if the envelope is present.
//
// This is possible because we can distinguish an envelope from a bare request
// struct by looking at the first byte and the length of the message.
//
// 1. A message of length 1 containing only 0x00 can only be an empty struct.
// 0x00 is the type ID for STOP, indicating the end of the struct.
//
// 2. A message of length >1 starting with 0x00 can only be a non-strict
// envelope (not versioned), assuming the message name is less than 16MB long.
// In this case, the first four bytes indicate the length of the method name,
// which is unlikely to overflow into the high byte.
//
// 3. A message of length >1, where the first byte is <0 can only be a strict envelope.
// The MSB indicates that the message is versioned. Reading the first two bytes
// and masking out the MSB indicates the version number.
// At this time, there is only one version.
//
// 4. A message of length >1, where the first byte is >=0 can only be a bare
// struct starting with that field identifier. Valid field identifiers today
// are in the range 0x00-0x0f. There is some chance that a future version of
// the protocol will add more field types, but it is very unlikely that the
// field type will flow into the MSB (128 type identifiers, starting with the
// 15 valid types today).
func (p *Protocol) DecodeRequest(et wire.EnvelopeType, r io.ReaderAt) (wire.Value, Responder, error) {
	var buf [2]byte

	// If we fail to read two bytes, the only possible valid value is the empty struct.
	if count, _ := r.ReadAt(buf[0:2], 0); count < 2 {
		val, err := p.Decode(r, wire.TStruct)
		return val, NoEnvelopeResponder, err
	}

	// If length > 1, 0x00 is only a valid preamble for a non-strict enveloped request.
	if buf[0] == 0x00 {
		e, err := p.DecodeEnveloped(r)
		if err != nil {
			return wire.Value{}, NoEnvelopeResponder, err
		}
		if e.Type != et {
			return wire.Value{}, NoEnvelopeResponder, errUnexpectedEnvelopeType(e.Type)
		}
		return e.Value, &EnvelopeV0Responder{
			Name:  e.Name,
			SeqID: e.SeqID,
		}, nil
	}

	// Only strict (versioned) envelopes begin with the most significant bit set.
	// This could only be confused for a type identifier greater than 127
	// (beyond the 15 Thrift has at time of writing), or a message name longer
	// than 16MB.
	if buf[0]&0x80 > 0 {
		e, err := p.DecodeEnveloped(r)
		if err != nil {
			return wire.Value{}, NoEnvelopeResponder, err
		}
		if e.Type != et {
			return wire.Value{}, NoEnvelopeResponder, errUnexpectedEnvelopeType(e.Type)
		}
		return e.Value, &EnvelopeV1Responder{
			Name:  e.Name,
			SeqID: e.SeqID,
		}, nil
	}

	// All other patterns are either bare structs or invalid.
	// We delegate to the struct decoder to distinguish invalid type
	// identifiers, outside the 0-15 range.
	val, err := p.Decode(r, wire.TStruct)
	return val, NoEnvelopeResponder, err
}

// ReadRequest reads off the request envelope (if present) from the Reader and
// returns a stream.Reader for the caller to read off the remaining un-enveloped
// request struct. In addition, ReadRequest also returns a stream.ResponseWriter
// for writing the response with an envelope style that matches the request.
// This allows a Thrift request handler to transparently accept requests
// regardless of whether the caller submits an envelope.
// The caller specifies the expected envelope type, one of OneWay or Unary, on
// which the decoder asserts if the envelope is present.
//
// This is possible because we can distinguish an envelope from a bare request
// struct by looking at the first byte and the length of the message.
//
// 1. A message of length 1 containing only 0x00 can only be an empty struct.
// 0x00 is the type ID for STOP, indicating the end of the struct.
//
// 2. A message of length >1 starting with 0x00 can only be a non-strict
// envelope (not versioned), assuming the message name is less than 16MB long.
// In this case, the first four bytes indicate the length of the method name,
// which is unlikely to overflow into the high byte.
//
// 3. A message of length >1, where the first byte is <0 can only be a strict envelope.
// The MSB indicates that the message is versioned. Reading the first two bytes
// and masking out the MSB indicates the version number.
// At this time, there is only one version.
//
// 4. A message of length >1, where the first byte is >=0 can only be a bare
// struct starting with that field identifier. Valid field identifiers today
// are in the range 0x00-0x0f. There is some chance that a future version of
// the protocol will add more field types, but it is very unlikely that the
// field type will flow into the MSB (128 type identifiers, starting with the
// 15 valid types today).
//
// Callers must call Close() on the stream.Reader once finished.
func (p *Protocol) Handle(
	ctx context.Context,
	et wire.EnvelopeType,
	r io.Reader,
	h stream.CallHandler,
) (stream.ResponseWriter, stream.Enveloper, error) {
	var buf [2]byte

	call := stream.Call{Request: p.Reader(bytes.NewReader(buf[:]))}
	if count, _ := r.Read(buf[0:2]); count < 2 {
		ev, err := h.HandleCall(ctx, &call)
		return NoEnvelopeResponder, ev, err
	}

	// Reset the Reader to allow for properly reading the envelope, if it exists.
	r = io.MultiReader(bytes.NewReader(buf[:]), r)
	sr := p.Reader(r)
	defer sr.Close()

	call = stream.Call{Request: sr}
	if buf[0] == 0x00 || buf[0]&0x80 > 0 {
		var responder stream.ResponseWriter = NoEnvelopeResponder
		eh, err := p.readEnvelopeHeader(sr, et)
		if err != nil {
			return responder, nil, err
		}
		defer sr.ReadEnvelopeEnd()

		switch {
		case buf[0] == 0x00:
			responder = &EnvelopeV0Responder{
				Name:  eh.Name,
				SeqID: eh.SeqID,
			}
		case buf[0]&0x80 > 0:
			responder = &EnvelopeV1Responder{
				Name:  eh.Name,
				SeqID: eh.SeqID,
			}
		default:
			return responder, nil, fmt.Errorf("should never happen")
		}

		ev, err := h.HandleCall(ctx, &call)
		if err != nil {
			return NoEnvelopeResponder, nil, err
		}
		return responder, ev, nil
	}

	// For anything else, the request is either not-enveloped or invalid, let the
	// bodyFunc manage that data.
	ev, err := h.HandleCall(ctx, &call)
	return NoEnvelopeResponder, ev, err
}

func (p *Protocol) readEnvelopeHeader(sr stream.Reader, et wire.EnvelopeType) (stream.EnvelopeHeader, error) {
	eh, err := sr.ReadEnvelopeBegin()
	if err != nil {
		return eh, err
	}
	if eh.Type != et {
		return eh, errUnexpectedEnvelopeType(eh.Type)
	}
	return eh, err
}

type errUnexpectedEnvelopeType wire.EnvelopeType

func (e errUnexpectedEnvelopeType) Error() string {
	return fmt.Sprintf("unexpected envelope type: %v", wire.EnvelopeType(e))
}
