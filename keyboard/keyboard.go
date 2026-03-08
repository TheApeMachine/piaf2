package keyboard

import (
	"io"

	"github.com/theapemachine/piaf/event"
)

type keyState uint8

const (
	stateNormal keyState = iota
	stateEscape
	stateCSI
	stateCSIParam
)

/*
Keyboard parses raw terminal input and emits event wire format.
Implements io.ReadWriteCloser: Write receives raw bytes, Read yields encoded events.
*/
type Keyboard struct {
	output    []byte
	readOff   int
	state     keyState
	csiParam  []byte
}

/*
keyboardOpts configures Keyboard.
*/
type keyboardOpts func(*Keyboard)

/*
NewKeyboard creates a new Keyboard.
*/
func NewKeyboard(opts ...keyboardOpts) *Keyboard {
	keyboard := &Keyboard{}

	for _, opt := range opts {
		opt(keyboard)
	}

	return keyboard
}

/*
Read implements the io.Reader interface.
Returns event wire format; EOF when drained.
*/
func (keyboard *Keyboard) Read(p []byte) (n int, err error) {
	if keyboard.readOff >= len(keyboard.output) {
		return 0, io.EOF
	}

	n = copy(p, keyboard.output[keyboard.readOff:])
	keyboard.readOff += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Parses raw terminal bytes, buffers encoded events for Read.
*/
func (keyboard *Keyboard) Write(p []byte) (n int, err error) {
	for _, b := range p {
		keyboard.processByte(b)
	}

	if keyboard.state != stateNormal {
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyEsc)
		keyboard.state = stateNormal
	}

	return len(p), nil
}

/*
Close implements the io.Closer interface.
*/
func (keyboard *Keyboard) Close() error {
	return nil
}

func (keyboard *Keyboard) processByte(b byte) {
	switch keyboard.state {
	case stateNormal:
		keyboard.handleNormal(b)
	case stateEscape:
		keyboard.handleEscape(b)
	case stateCSI:
		keyboard.handleCSI(b)
	case stateCSIParam:
		keyboard.handleCSIParam(b)
	}
}

func (keyboard *Keyboard) handleNormal(b byte) {
	if b == 0x1b {
		keyboard.state = stateEscape
		return
	}

	if b >= 0x20 {
		keyboard.output = event.EncodeRune(keyboard.output, rune(b))
		return
	}

	switch b {
	case 0x7f, 0x08:
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyBackspace)
	case '\r', '\n':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyEnter)
	case 0x03:
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyQuit)
	}
}

func (keyboard *Keyboard) handleEscape(b byte) {
	if b == '[' {
		keyboard.state = stateCSI
		return
	}

	keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyEsc)
	keyboard.state = stateNormal
}

func (keyboard *Keyboard) handleCSI(b byte) {
	if b >= 0x30 && b <= 0x3f {
		keyboard.csiParam = append(keyboard.csiParam[:0], b)
		keyboard.state = stateCSIParam
		return
	}

	keyboard.state = stateNormal

	switch b {
	case 'A':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyUp)
	case 'B':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyDown)
	case 'C':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyRight)
	case 'D':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyLeft)
	}
}

func (keyboard *Keyboard) handleCSIParam(b byte) {
	if b >= 0x30 && b <= 0x3f {
		keyboard.csiParam = append(keyboard.csiParam, b)
		return
	}

	keyboard.state = stateNormal

	switch b {
	case 'A':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyUp)
	case 'B':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyDown)
	case 'C':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyRight)
	case 'D':
		keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyLeft)
	case '~':
		if len(keyboard.csiParam) == 1 && keyboard.csiParam[0] == '3' {
			keyboard.output = event.EncodeSpecial(keyboard.output, event.KeyBackspace)
		}
	}

	keyboard.csiParam = keyboard.csiParam[:0]
}
