package tui

import (
	"encoding/binary"
	"io"
)

/*
Frame represents a single screen state for rendering.
Implements io.ReadWriteCloser: Write decodes wire format, Read encodes it.
Wire format is a compact binary layout, pending Cap'n Proto migration.
*/
type Frame struct {
	Lines     []string
	CursorRow uint32
	CursorCol uint32
	Mode      string
	Width     uint32
	Height    uint32

	readBuf    []byte
	readOffset int
}

/*
Read implements the io.Reader interface.
Encodes the Frame to wire format and copies into p.
*/
func (frame *Frame) Read(p []byte) (n int, err error) {
	if frame.readOffset >= len(frame.readBuf) {
		if len(frame.readBuf) > 0 {
			return 0, io.EOF
		}

		size := 4
		for _, line := range frame.Lines {
			size += 4 + len(line)
		}
		size += 4 + 4 + 4 + len(frame.Mode) + 4 + 4

		frame.readBuf = make([]byte, 0, size)
		buf := make([]byte, 4)

		binary.LittleEndian.PutUint32(buf, uint32(len(frame.Lines)))
		frame.readBuf = append(frame.readBuf, buf...)

		for _, line := range frame.Lines {
			binary.LittleEndian.PutUint32(buf, uint32(len(line)))
			frame.readBuf = append(frame.readBuf, buf...)
			frame.readBuf = append(frame.readBuf, line...)
		}

		binary.LittleEndian.PutUint32(buf, frame.CursorRow)
		frame.readBuf = append(frame.readBuf, buf...)
		binary.LittleEndian.PutUint32(buf, frame.CursorCol)
		frame.readBuf = append(frame.readBuf, buf...)
		binary.LittleEndian.PutUint32(buf, uint32(len(frame.Mode)))
		frame.readBuf = append(frame.readBuf, buf...)
		frame.readBuf = append(frame.readBuf, frame.Mode...)
		binary.LittleEndian.PutUint32(buf, frame.Width)
		frame.readBuf = append(frame.readBuf, buf...)
		binary.LittleEndian.PutUint32(buf, frame.Height)
		frame.readBuf = append(frame.readBuf, buf...)

		frame.readOffset = 0

		if len(frame.readBuf) == 0 {
			return 0, nil
		}
	}

	n = copy(p, frame.readBuf[frame.readOffset:])
	frame.readOffset += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Decodes wire format from p into the Frame.
*/
func (frame *Frame) Write(p []byte) (n int, err error) {
	if len(p) < 4 {
		if len(p) == 0 {
			return 0, nil
		}

		return 0, io.ErrShortBuffer
	}

	offset := 0
	numLines := binary.LittleEndian.Uint32(p[offset:])
	offset += 4

	frame.Lines = make([]string, 0, numLines)
	for index := uint32(0); index < numLines; index++ {
		if offset+4 > len(p) {
			return 0, io.ErrShortBuffer
		}

		lineLen := binary.LittleEndian.Uint32(p[offset:])
		offset += 4
		if offset+int(lineLen) > len(p) {
			return 0, io.ErrShortBuffer
		}

		frame.Lines = append(frame.Lines, string(p[offset:offset+int(lineLen)]))
		offset += int(lineLen)
	}

	if offset+20 > len(p) {
		return 0, io.ErrShortBuffer
	}

	frame.CursorRow = binary.LittleEndian.Uint32(p[offset:])
	offset += 4
	frame.CursorCol = binary.LittleEndian.Uint32(p[offset:])
	offset += 4

	modeLen := binary.LittleEndian.Uint32(p[offset:])
	offset += 4
	if offset+int(modeLen)+8 > len(p) {
		return 0, io.ErrShortBuffer
	}

	frame.Mode = string(p[offset : offset+int(modeLen)])
	offset += int(modeLen)

	frame.Width = binary.LittleEndian.Uint32(p[offset:])
	offset += 4
	frame.Height = binary.LittleEndian.Uint32(p[offset:])

	frame.readBuf = nil
	frame.readOffset = 0

	return len(p), nil
}

/*
Close implements the io.Closer interface.
*/
func (frame *Frame) Close() error {
	return nil
}
