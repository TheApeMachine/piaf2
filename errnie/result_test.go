package errnie

import (
	"errors"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestOk(t *testing.T) {
	convey.Convey("Given Ok constructor", t, func() {
		convey.Convey("When creating Ok with a value", func() {
			result := Ok(42)

			convey.Convey("It should unwrap to the value and nil error", func() {
				value, err := result.Unwrap()
				
				convey.So(err, convey.ShouldBeNil)
				convey.So(value, convey.ShouldEqual, 42)
			})
		})
	})
}

func TestFail(t *testing.T) {
	convey.Convey("Given Fail constructor", t, func() {
		convey.Convey("When creating Fail with an error", func() {
			sentinel := errors.New("fail")
			result := Fail[int](sentinel)

			convey.Convey("It should return the error from Err", func() {
				convey.So(result.Err(), convey.ShouldEqual, sentinel)
			})

			convey.Convey("It should unwrap to zero value and error", func() {
				value, err := result.Unwrap()

				convey.So(err, convey.ShouldEqual, sentinel)
				convey.So(value, convey.ShouldEqual, 0)
			})
		})
	})
}

func TestTry(t *testing.T) {
	convey.Convey("Given Try constructor", t, func() {
		convey.Convey("When value and nil error", func() {
			result := Try(100, nil)

			convey.Convey("It should produce Ok", func() {
				value, err := result.Unwrap()
				convey.So(err, convey.ShouldBeNil)
				convey.So(value, convey.ShouldEqual, 100)
			})
		})

		convey.Convey("When value and non-nil error", func() {
			sentinel := errors.New("try failed")
			result := Try(0, sentinel)

			convey.Convey("It should produce Fail", func() {
				convey.So(result.Err(), convey.ShouldEqual, sentinel)
			})
		})
	})
}

func TestResultMap(t *testing.T) {
	convey.Convey("Given a Result", t, func() {
		convey.Convey("When Ok value and Map is applied", func() {
			result := Ok(10).Map(func(value int) int { return value * 2 })

			convey.Convey("It should transform the value", func() {
				value, err := result.Unwrap()
				convey.So(err, convey.ShouldBeNil)
				convey.So(value, convey.ShouldEqual, 20)
			})
		})

		convey.Convey("When Fail and Map is applied", func() {
			sentinel := errors.New("map fail")
			result := Fail[int](sentinel).Map(func(value int) int { return value + 1 })

			convey.Convey("It should preserve the error", func() {
				convey.So(result.Err(), convey.ShouldEqual, sentinel)
			})
		})
	})
}

func TestFlatMap(t *testing.T) {
	convey.Convey("Given FlatMap", t, func() {
		convey.Convey("When Ok value and fn succeeds", func() {
			result := FlatMap(Ok(5), func(value int) (string, error) {
				return "five", nil
			})

			convey.Convey("It should produce Ok with transformed value", func() {
				value, err := result.Unwrap()
				convey.So(err, convey.ShouldBeNil)
				convey.So(value, convey.ShouldEqual, "five")
			})
		})

		convey.Convey("When Ok value and fn returns error", func() {
			sentinel := errors.New("flatmap fn failed")

			result := FlatMap(Ok(5), func(value int) (string, error) {
				return "", sentinel
			})

			convey.Convey("It should produce Fail", func() {
				convey.So(result.Err(), convey.ShouldEqual, sentinel)
			})
		})

		convey.Convey("When Fail and FlatMap is applied", func() {
			sentinel := errors.New("original fail")

			result := FlatMap(Fail[int](sentinel), func(value int) (string, error) {
				return "never", nil
			})

			convey.Convey("It should preserve the original error", func() {
				convey.So(result.Err(), convey.ShouldEqual, sentinel)
			})
		})
	})
}

func BenchmarkOk(b *testing.B) {
	for b.Loop() {
		_ = Ok(42)
	}
}

func BenchmarkFail(b *testing.B) {
	sentinel := errors.New("bench")
	for b.Loop() {
		_ = Fail[int](sentinel)
	}
}

func BenchmarkTryOk(b *testing.B) {
	for b.Loop() {
		_ = Try(42, nil)
	}
}

func BenchmarkMap(b *testing.B) {
	result := Ok(10)
	double := func(value int) int { return value * 2 }

	for b.Loop() {
		_ = result.Map(double)
	}
}

func BenchmarkFlatMap(b *testing.B) {
	result := Ok(5)
	fn := func(value int) (int, error) { return value * 2, nil }

	for b.Loop() {
		_ = FlatMap(result, fn)
	}
}
