package errnie

import (
	"errors"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestLift(t *testing.T) {
	convey.Convey("Given Lift", t, func() {
		convey.Convey("When lifting a function that succeeds", func() {
			lifted := Lift(func() (int, error) { return 7, nil })
			result := lifted()

			convey.Convey("It should return Ok with the value", func() {
				value, err := result.Unwrap()

				convey.So(err, convey.ShouldBeNil)
				convey.So(value, convey.ShouldEqual, 7)
			})
		})

		convey.Convey("When lifting a function that fails", func() {
			sentinel := errors.New("lift failed")
			lifted := Lift(func() (int, error) { return 0, sentinel })
			result := lifted()

			convey.Convey("It should return Fail with the error", func() {
				convey.So(result.Err(), convey.ShouldEqual, sentinel)
			})
		})

		convey.Convey("When the lifted function is invoked multiple times", func() {
			counter := 0
			lifted := Lift(func() (int, error) {
				counter++
				return counter, nil
			})

			convey.Convey("It should invoke the underlying function each time", func() {
				first, _ := lifted().Unwrap()
				second, _ := lifted().Unwrap()

				convey.So(first, convey.ShouldEqual, 1)
				convey.So(second, convey.ShouldEqual, 2)
			})
		})
	})
}

func BenchmarkLift(b *testing.B) {
	fn := func() (int, error) { return 42, nil }
	lifted := Lift(fn)

	for b.Loop() {
		_ = lifted()
	}
}
