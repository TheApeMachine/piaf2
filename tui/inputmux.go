package tui

import (
"io"
"sync"
)

/*
SentinelRefresh is the byte written when the refresh channel fires.
App.Write treats it as a signal to pump output without keyboard input.
*/
const SentinelRefresh = 0xFE

/*
SentinelQuit is the byte returned when the quit pipe has data.
The main loop breaks without writing to the App.
*/
const SentinelQuit = 0xFF

/*
InputMux multiplexes a blocking stdin Reader and a refresh channel.
Implements io.Reader: Read blocks until stdin has data (returned as-is) or refresh fires (returns sentinel).
Enables streaming updates to flow through Write to the App without extra methods.
*/
type InputMux struct {
	stdin    io.Reader
	refresh  <-chan struct{}
	quit     io.Reader
	buf      []byte
	stdinCh  chan []byte
	errCh    chan error
	quitCh   chan struct{}
	once     sync.Once
	leftover []byte
}

/*
InputMuxOpts configures InputMux.
*/
type inputMuxOpts func(*InputMux)

/*
InputMuxWithStdin sets the stdin source.
*/
func InputMuxWithStdin(stdin io.Reader) inputMuxOpts {
	return func(mux *InputMux) {
		mux.stdin = stdin
	}
}

/*
InputMuxWithRefresh sets the channel that signals streaming updates.
*/
func InputMuxWithRefresh(ch <-chan struct{}) inputMuxOpts {
	return func(mux *InputMux) {
		mux.refresh = ch
	}
}

/*
InputMuxWithQuit sets the reader that signals quit (e.g. pipe read end).
When it has data, Read returns SentinelQuit.
*/
func InputMuxWithQuit(quit io.Reader) inputMuxOpts {
	return func(mux *InputMux) {
		mux.quit = quit
	}
}

/*
NewInputMux creates an InputMux.
*/
func NewInputMux(opts ...inputMuxOpts) *InputMux {
	mux := &InputMux{
		buf: make([]byte, 1),
	}

	for _, opt := range opts {
		opt(mux)
	}

	return mux
}

func (mux *InputMux) init() {
	if mux.stdin != nil {
		mux.stdinCh = make(chan []byte)
		mux.errCh = make(chan error, 1)
		go func() {
			for {
				b := make([]byte, 1024)
				n, err := mux.stdin.Read(b)
				if n > 0 {
					mux.stdinCh <- b[:n]
				}
				if err != nil {
					mux.errCh <- err
					return
				}
			}
		}()
	}

	if mux.quit != nil {
		mux.quitCh = make(chan struct{})
		go func() {
			b := make([]byte, 1)
			mux.quit.Read(b)
			mux.quitCh <- struct{}{}
		}()
	}
}

/*
Read implements io.Reader.
Returns stdin bytes or a single refresh sentinel byte when the channel fires.
*/
func (mux *InputMux) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	mux.once.Do(mux.init)

	if len(mux.leftover) > 0 {
		n := copy(p, mux.leftover)
		mux.leftover = mux.leftover[n:]
		return n, nil
	}

	// Dynamic select based on available channels
	// But we can simplify by assigning nil to channels we don't use
var stdinCh <-chan []byte = mux.stdinCh
var errCh <-chan error = mux.errCh
var quitCh <-chan struct{} = mux.quitCh
var refreshCh <-chan struct{} = mux.refresh

select {
case b := <-stdinCh:
n := copy(p, b)
if n < len(b) {
mux.leftover = b[n:]
}
return n, nil
case err := <-errCh:
return 0, err
case <-quitCh:
p[0] = SentinelQuit
return 1, nil
case <-refreshCh:
p[0] = SentinelRefresh
return 1, nil
}
}

/*
Write implements io.Writer.
InputMux is read-only; Write discards.
*/
func (mux *InputMux) Write(p []byte) (int, error) {
return len(p), nil
}

/*
Close implements io.Closer.
*/
func (mux *InputMux) Close() error {
return nil
}
