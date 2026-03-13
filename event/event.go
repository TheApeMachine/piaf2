package event

import (
	"unicode/utf8"
)

/*
Key identifies special keys in the event wire format.
*/
type Key byte

const (
	KeyEsc       Key = 0
	KeyUp        Key = 1
	KeyDown      Key = 2
	KeyLeft      Key = 3
	KeyRight     Key = 4
	KeyBackspace Key = 5
	KeyEnter     Key = 6
	KeyQuit      Key = 7
	KeyRefresh   Key = 8
)

const (
	TypeRune    byte = 0
	TypeSpecial byte = 1
)

/*
EncodeRune appends a rune event to buf, returns the new length.
*/
func EncodeRune(buf []byte, r rune) []byte {
	buf = append(buf, TypeRune)
	runebuf := make([]byte, utf8.UTFMax)
	length := utf8.EncodeRune(runebuf, r)
	return append(buf, runebuf[:length]...)
}

/*
EncodeSpecial appends a special key event to buf.
*/
func EncodeSpecial(buf []byte, key Key) []byte {
	return append(buf, TypeSpecial, byte(key))
}

/*
ParseEvent decodes one event from data. Returns isRune, runeVal, key, and bytes consumed.
*/
func ParseEvent(data []byte) (isRune bool, runeVal rune, key Key, size int) {
	if len(data) < 1 {
		return false, 0, 0, 0
	}

	if data[0] == TypeRune {
		if len(data) < 2 {
			return true, utf8.RuneError, 0, 0
		}

		r, n := utf8.DecodeRune(data[1:])
		if n == 0 {
			return true, utf8.RuneError, 0, 0
		}

		return true, r, 0, 1 + n
	}

	if data[0] == TypeSpecial && len(data) >= 2 {
		return false, 0, Key(data[1]), 2
	}

	return false, 0, 0, 0
}
