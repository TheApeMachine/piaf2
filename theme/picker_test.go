package theme

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestNewPicker(t *testing.T) {
	convey.Convey("Given NewPicker", t, func() {
		th := Default()
		picker := NewPicker(th)

		convey.Convey("When created", func() {

			convey.Convey("It should start at cursor 0 channel 0", func() {
				convey.So(picker.Cursor(), convey.ShouldEqual, 0)
				convey.So(picker.Channel(), convey.ShouldEqual, 0)
			})
		})
	})
}

func TestPickerNavigation(t *testing.T) {
	convey.Convey("Given a Picker", t, func() {
		th := Default()
		picker := NewPicker(th)

		convey.Convey("When MoveDown is called", func() {
			picker.MoveDown()

			convey.Convey("It should advance the cursor", func() {
				convey.So(picker.Cursor(), convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When MoveUp is called at 0", func() {
			picker.MoveUp()

			convey.Convey("It should stay at 0", func() {
				convey.So(picker.Cursor(), convey.ShouldEqual, 0)
			})
		})

		convey.Convey("When CycleChannel is called", func() {
			picker.CycleChannel()

			convey.Convey("It should advance to channel 1", func() {
				convey.So(picker.Channel(), convey.ShouldEqual, 1)
			})
		})

		convey.Convey("When CycleChannel wraps around", func() {
			picker.CycleChannel()
			picker.CycleChannel()
			picker.CycleChannel()

			convey.Convey("It should return to channel 0", func() {
				convey.So(picker.Channel(), convey.ShouldEqual, 0)
			})
		})
	})
}

func TestPickerColorAdjust(t *testing.T) {
	convey.Convey("Given a Picker on the Brand role, channel R", t, func() {
		th := Default()
		picker := NewPicker(th)
		original := th.UI.Brand.R

		convey.Convey("When Increase is called", func() {
			picker.Increase(10)

			convey.Convey("It should increase the R channel", func() {
				convey.So(th.UI.Brand.R, convey.ShouldEqual, original+10)
			})
		})

		convey.Convey("When Decrease is called", func() {
			picker.Decrease(10)

			convey.Convey("It should decrease the R channel", func() {
				convey.So(th.UI.Brand.R, convey.ShouldEqual, original-10)
			})
		})

		convey.Convey("When Increase overflows", func() {
			th.UI.Brand.R = 250
			picker.Increase(10)

			convey.Convey("It should clamp to 255", func() {
				convey.So(th.UI.Brand.R, convey.ShouldEqual, 255)
			})
		})

		convey.Convey("When Decrease underflows", func() {
			th.UI.Brand.R = 3
			picker.Decrease(10)

			convey.Convey("It should clamp to 0", func() {
				convey.So(th.UI.Brand.R, convey.ShouldEqual, 0)
			})
		})
	})
}

func TestPickerOverlay(t *testing.T) {
	convey.Convey("Given a Picker overlay", t, func() {
		th := Default()
		picker := NewPicker(th)
		bg := make([]string, 30)

		for index := range bg {
			bg[index] = ""
		}

		convey.Convey("When Overlay is rendered", func() {
			lines := picker.Overlay(bg, 80, 30)

			convey.Convey("It should produce output lines", func() {
				convey.So(len(lines), convey.ShouldBeGreaterThan, 0)
			})

			convey.Convey("It should contain the theme name", func() {
				found := false
				for _, line := range lines {
					if containsVisible(line, "default") {
						found = true
						break
					}
				}
				convey.So(found, convey.ShouldBeTrue)
			})

			convey.Convey("It should contain role names", func() {
				found := false
				for _, line := range lines {
					if containsVisible(line, "Brand") {
						found = true
						break
					}
				}
				convey.So(found, convey.ShouldBeTrue)
			})
		})
	})
}

func TestPickerReadWriteClose(t *testing.T) {
	convey.Convey("Given a Picker", t, func() {
		picker := NewPicker(Default())

		convey.Convey("When Write is called", func() {
			n, err := picker.Write([]byte("data"))

			convey.Convey("It should accept and discard", func() {
				convey.So(n, convey.ShouldEqual, 4)
				convey.So(err, convey.ShouldBeNil)
			})
		})

		convey.Convey("When Close is called", func() {
			err := picker.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func containsVisible(s, substr string) bool {
	cleaned := ""
	inEscape := false

	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}

		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		cleaned += string(r)
	}

	for index := 0; index <= len(cleaned)-len(substr); index++ {
		if cleaned[index:index+len(substr)] == substr {
			return true
		}
	}

	return false
}

func BenchmarkPickerOverlay(b *testing.B) {
	th := Default()
	picker := NewPicker(th)
	bg := make([]string, 30)

	for index := 0; index < b.N; index++ {
		picker.Overlay(bg, 80, 30)
	}
}
