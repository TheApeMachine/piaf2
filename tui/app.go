package tui

import (
	"bytes"
	"io"

	"github.com/theapemachine/piaf/editor"
	"github.com/theapemachine/piaf/keyboard"
)

/*
App wires Keyboard → Editor → Renderer and exposes io.ReadWriteCloser.
Write receives raw terminal bytes, Read yields ANSI output.
*/
type App struct {
	keyboard *keyboard.Keyboard
	editor   *editor.Editor
	renderer *Renderer
	output   []byte
	readOff  int
}

/*
appOpts configures App.
*/
type appOpts func(*App)

/*
NewApp creates a new App with Keyboard, Editor, and Renderer wired.
*/
func NewApp(opts ...appOpts) *App {
	app := &App{
		keyboard: keyboard.NewKeyboard(),
		editor:   editor.NewEditor(),
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
Read implements the io.Reader interface.
Returns buffered ANSI output; EOF when drained.
*/
func (app *App) Read(p []byte) (n int, err error) {
	if app.readOff >= len(app.output) {
		return 0, io.EOF
	}

	n = copy(p, app.output[app.readOff:])
	app.readOff += n

	return n, nil
}

/*
Write implements the io.Writer interface.
Routes raw bytes to Keyboard, pumps the pipeline, buffers ANSI for Read.
*/
func (app *App) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return len(p), nil
	}

	app.keyboard.Write(p)
	app.pump()

	return len(p), nil
}

/*
Close implements the io.Closer interface.
Closes the Renderer to restore the terminal.
*/
func (app *App) Close() error {
	return app.renderer.Close()
}

/*
QuitRequested returns true if the user executed a quit command (:q, :q!, :wq).
*/
func (app *App) QuitRequested() bool {
	return app.editor.QuitRequested()
}

func (app *App) pump() {
	io.Copy(app.editor, app.keyboard)
	io.Copy(app.renderer, app.editor)

	buf := &bytes.Buffer{}
	io.Copy(buf, app.renderer)

	app.output = append(app.output[:0], buf.Bytes()...)
	app.readOff = 0
}
