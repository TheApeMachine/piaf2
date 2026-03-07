package tui

/*
App is the main application struct.
It wires together the Editor and exposes a simple io.ReadWriteCloser
interface for the command layer to drive.
*/
type App struct {
	editor *Editor
}

/*
appOpts configures App with options.
*/
type appOpts func(*App)

/*
NewApp creates a new App instance with a default Editor.
*/
func NewApp(opts ...appOpts) *App {
	app := &App{
		editor: NewEditor(),
	}

	for _, opt := range opts {
		opt(app)
	}

	return app
}

/*
Read implements the io.Reader interface.
Returns rendered ANSI output from the current editor state.
*/
func (app *App) Read(p []byte) (n int, err error) {
	return app.editor.Read(p)
}

/*
Write implements the io.Writer interface.
Routes input bytes to the editor for processing.
*/
func (app *App) Write(p []byte) (n int, err error) {
	return app.editor.Write(p)
}

/*
Close implements the io.Closer interface.
Closes the editor and restores the terminal.
*/
func (app *App) Close() error {
	return app.editor.Close()
}

/*
AppWithEditor configures App with a custom Editor.
*/
func AppWithEditor(editor *Editor) appOpts {
	return func(app *App) {
		app.editor = editor
	}
}
