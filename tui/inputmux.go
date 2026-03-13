package tui

import (
	"io"
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
	stdin   io.Reader
	refresh <-chan struct{}
	quit    io.Reader
	buf     []byte
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

/*
Read implements io.Reader.
Returns stdin bytes or a single refresh sentinel byte when the channel fires.
*/
func (mux *InputMux) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	stdinCh := make(chan result, 1)
	if mux.stdin != nil {
		go func() {
			count, err := mux.stdin.Read(p)
			stdinCh <- result{count: count, err: err}
		}()
	}

	quitCh := make(chan struct{}, 1)
	if mux.quit != nil {
		go func() {
			mux.quit.Read(mux.buf)
			quitCh <- struct{}{}
		}()
	}

	refreshCh := mux.refresh
	if refreshCh == nil {
		refreshCh = make(chan struct{})
	}

	quitChRead := quitCh
	if mux.quit == nil {
		quitChRead = make(chan struct{})
	}

	if mux.stdin != nil {
		select {
		case res := <-stdinCh:
			return res.count, res.err
		case <-quitChRead:
			p[0] = SentinelQuit
			return 1, nil
		case <-refreshCh:
			p[0] = SentinelRefresh
			return 1, nil
		}
	}

	if mux.quit != nil {
		select {
		case <-quitChRead:
			p[0] = SentinelQuit
			return 1, nil
		case <-refreshCh:
			p[0] = SentinelRefresh
			return 1, nil
		}
	}

	if mux.refresh != nil {
		<-mux.refresh
		p[0] = SentinelRefresh
		return 1, nil
	}

	return 0, io.EOF
}

type result struct {
	count int
	err   error
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
