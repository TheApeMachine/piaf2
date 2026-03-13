package tui

import (
	"bytes"
	"io"

	"github.com/theapemachine/piaf/editor"
	"github.com/theapemachine/piaf/keyboard"
	"github.com/theapemachine/piaf/wire"
)

/*
App wires Keyboard → Editor → Renderer and exposes io.ReadWriteCloser.
Write receives raw bytes (stdin or SentinelRefresh); Read yields ANSI output.
*/
type App struct {
	keyboard   *keyboard.Keyboard
	editor     *editor.Editor
	renderer   *Renderer
	output     []byte
	readOff    int
	closed     bool
	quitWriter io.Writer
}

/*
appOpts configures App.
*/
type appOpts func(*App)

/*
NewApp creates a new App with Keyboard, Editor, and Renderer wired.
*/
func NewApp(opts ...appOpts) *App {
	streamCh := make(chan struct{}, 16)
	app := &App{
		keyboard: keyboard.NewKeyboard(),
		editor:   editor.NewEditor(editor.EditorWithStreamUpdates(streamCh)),
		renderer: NewRenderer(),
	}

	for _, opt := range opts {
		opt(app)
	}

	app.pump()

	return app
}

/*
AppWithEditor configures App with a custom Editor.
*/
func AppWithEditor(ed *editor.Editor) appOpts {
	return func(app *App) {
		app.editor = ed
	}
}

/*
AppWithQuitWriter sets the writer to signal when a Frame has Quit.
Enables the main loop to break via InputMuxWithQuit(pipe.Read) without polling.
*/
func AppWithQuitWriter(w io.Writer) appOpts {
	return func(app *App) {
		app.quitWriter = w
	}
}

/*
Read implements the io.Reader interface.
Returns buffered ANSI output; EOF when drained or when a Frame had Quit.
*/
func (app *App) Read(p []byte) (n int, err error) {
	if app.closed {
		return 0, io.EOF
	}

	if app.readOff >= len(app.output) {
		return 0, io.EOF
	}

	n = copy(p, app.output[app.readOff:])
	app.readOff += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Routes raw bytes to Keyboard; 0xFE (SentinelRefresh) yields KeyRefresh event.
*/
func (app *App) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	app.keyboard.Write(p)
	app.pump()

	return len(p), nil
}

/*
Closed returns true if the app has received a quit request.
*/
func (app *App) Closed() bool {
	return app.closed
}

/*
Close implements the io.Closer interface.
Closes the Renderer to restore the terminal.
*/
func (app *App) Close() error {
	return app.renderer.Close()
}

func (app *App) pump() {
	io.Copy(app.editor, app.keyboard)

	frameBytes, err := io.ReadAll(app.editor)
	if err != nil || len(frameBytes) == 0 {
		return
	}

	frame := &wire.Frame{}
	if _, err := frame.Write(frameBytes); err == nil && frame.Quit {
		app.closed = true
		if app.quitWriter != nil {
			app.quitWriter.Write([]byte{1})
		}
	}

	app.renderer.Write(frameBytes)
	buf := &bytes.Buffer{}
	io.Copy(buf, app.renderer)
	app.output = append(app.output[:0], buf.Bytes()...)
	app.readOff = 0
}
