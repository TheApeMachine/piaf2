package tui

/*
App is the main application struct.
*/
type App struct {
	renderer *Renderer
}

/*
opts configures App with options.
*/
type appOpts func(*App)

/*
NewApp creates a new App instance.
*/
func NewApp(opts ...appOpts) *App {
	a := &App{}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

/*
Read implements the io.Reader interface.
*/
func (a *App) Read(p []byte) (n int, err error) {
	return 0, nil
}

/*
Write implements the io.Writer interface.
*/
func (a *App) Write(p []byte) (n int, err error) {
	return len(p), nil
}

/*
Close implements the io.Closer interface.
*/
func (a *App) Close() error {
	return nil
}

/*
WithRenderer configures App with a Renderer.
*/
func AppWithRenderer(renderer *Renderer) appOpts {
	return func(app *App) {
		app.renderer = renderer
	}
}
