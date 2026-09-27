package decoder

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrTruncatedPayload = errors.New("payload Vio truncado")

type byteCursor struct {
	data   []byte
	offset int
}

func newByteCursor(data []byte) *byteCursor {
	return &byteCursor{data: data}
}

func (cursor *byteCursor) readUint16() (uint16, error) {
	value, err := cursor.readBytes(2)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(value), nil
}

func (cursor *byteCursor) readBytes(length int) ([]byte, error) {
	if length < 0 || cursor.offset+length > len(cursor.data) {
		return nil, fmt.Errorf("%w: offset=%d length=%d total=%d", ErrTruncatedPayload, cursor.offset, length, len(cursor.data))
	}

	value := cursor.data[cursor.offset : cursor.offset+length]
	cursor.offset += length
	return value, nil
}

func (cursor *byteCursor) remaining() []byte {
	return cursor.data[cursor.offset:]
}

func appendUint16(target []byte, value uint16) []byte {
	buffer := [2]byte{}
	binary.BigEndian.PutUint16(buffer[:], value)
	return append(target, buffer[:]...)
}
