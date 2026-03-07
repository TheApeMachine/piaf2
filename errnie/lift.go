package errnie

/*
Lift takes a function returning (T, error) and returns a func() Result[T].
*/
func Lift[T any](fn func() (T, error)) func() Result[T] {
	return func() Result[T] {
		return Try(fn())
	}
}
