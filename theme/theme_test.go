package theme

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestNewTheme(t *testing.T) {
	convey.Convey("Given NewTheme", t, func() {

		convey.Convey("When called", func() {
			th := NewTheme()

			convey.Convey("It should return a non-nil Theme with default name", func() {
				convey.So(th, convey.ShouldNotBeNil)
				convey.So(th.Name, convey.ShouldEqual, "default")
			})
		})
	})
}

func TestDefault(t *testing.T) {
	convey.Convey("Given Default", t, func() {

		convey.Convey("When called", func() {
			th := Default()

			convey.Convey("It should have the brand purple color", func() {
				convey.So(th.UI.Brand.R, convey.ShouldEqual, 108)
				convey.So(th.UI.Brand.G, convey.ShouldEqual, 80)
				convey.So(th.UI.Brand.B, convey.ShouldEqual, 255)
			})

			convey.Convey("It should produce correct ANSI foreground", func() {
				convey.So(th.FgBrand(), convey.ShouldEqual, "\033[38;2;108;80;255m")
			})

			convey.Convey("It should produce correct ANSI background", func() {
				convey.So(th.BgBrand(), convey.ShouldEqual, "\033[48;2;108;80;255m")
			})
		})
	})
}

func TestColor(t *testing.T) {
	convey.Convey("Given a Color", t, func() {
		color := Color{R: 255, G: 128, B: 0}

		convey.Convey("When Fg is called", func() {
			result := color.Fg()

			convey.Convey("It should produce a 24-bit ANSI foreground escape", func() {
				convey.So(result, convey.ShouldEqual, "\033[38;2;255;128;0m")
			})
		})

		convey.Convey("When Bg is called", func() {
			result := color.Bg()

			convey.Convey("It should produce a 24-bit ANSI background escape", func() {
				convey.So(result, convey.ShouldEqual, "\033[48;2;255;128;0m")
			})
		})
	})
}

func TestThemeReadWrite(t *testing.T) {
	convey.Convey("Given a Theme", t, func() {
		th := Default()

		convey.Convey("When Read is called", func() {
			data, err := io.ReadAll(th)

			convey.Convey("It should produce valid YAML", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(len(data), convey.ShouldBeGreaterThan, 0)
				convey.So(string(data), convey.ShouldContainSubstring, "name: default")
			})
		})

		convey.Convey("When Write is called with valid YAML", func() {
			target := &Theme{}
			yaml := `name: neon
ui:
  brand:
    r: 255
    g: 0
    b: 128
`
			n, err := target.Write([]byte(yaml))

			convey.Convey("It should deserialize the theme", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(n, convey.ShouldEqual, len(yaml))
				convey.So(target.Name, convey.ShouldEqual, "neon")
				convey.So(target.UI.Brand.R, convey.ShouldEqual, 255)
				convey.So(target.UI.Brand.G, convey.ShouldEqual, 0)
				convey.So(target.UI.Brand.B, convey.ShouldEqual, 128)
			})
		})

		convey.Convey("When Write is called with invalid YAML", func() {
			target := &Theme{}
			_, err := target.Write([]byte(":::invalid"))

			convey.Convey("It should return an error", func() {
				convey.So(err, convey.ShouldNotBeNil)
			})
		})

		convey.Convey("When Close is called", func() {
			err := th.Close()

			convey.Convey("It should return nil", func() {
				convey.So(err, convey.ShouldBeNil)
			})
		})
	})
}

func TestActive(t *testing.T) {
	convey.Convey("Given the global Active theme", t, func() {

		convey.Convey("When Active is called with no explicit set", func() {
			SetActive(nil)
			th := Active()

			convey.Convey("It should return the default theme", func() {
				convey.So(th, convey.ShouldNotBeNil)
				convey.So(th.Name, convey.ShouldEqual, "default")
			})
		})

		convey.Convey("When SetActive is called with a custom theme", func() {
			custom := &Theme{Name: "custom"}
			SetActive(custom)

			convey.Convey("Active should return the custom theme", func() {
				convey.So(Active().Name, convey.ShouldEqual, "custom")
			})
		})

		SetActive(nil)
	})
}

func TestThemeSaveLoad(t *testing.T) {
	convey.Convey("Given a Theme", t, func() {
		dir := t.TempDir()
		origDir := ThemesDir

		ThemesDir = func() string { return dir }
		defer func() { ThemesDir = origDir }()

		th := Default()
		th.Name = "test-save"

		convey.Convey("When Save is called", func() {
			err := th.Save()

			convey.Convey("It should write a file to the themes directory", func() {
				convey.So(err, convey.ShouldBeNil)
				_, statErr := os.Stat(filepath.Join(dir, "test-save.yml"))
				convey.So(statErr, convey.ShouldBeNil)
			})
		})

		convey.Convey("When Load is called on a saved theme", func() {
			th.Save()
			loaded, err := Load("test-save")

			convey.Convey("It should return the theme with matching colors", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(loaded.Name, convey.ShouldEqual, "test-save")
				convey.So(loaded.UI.Brand.R, convey.ShouldEqual, 108)
			})
		})

		convey.Convey("When Load is called on a missing theme", func() {
			_, err := Load("nonexistent")

			convey.Convey("It should return an error", func() {
				convey.So(err, convey.ShouldNotBeNil)
			})
		})
	})
}

func TestThemeRename(t *testing.T) {
	convey.Convey("Given a saved Theme", t, func() {
		dir := t.TempDir()
		origDir := ThemesDir

		ThemesDir = func() string { return dir }
		defer func() { ThemesDir = origDir }()

		th := Default()
		th.Name = "original"
		th.Save()

		convey.Convey("When Rename is called", func() {
			err := th.Rename("renamed")

			convey.Convey("It should update the name and create new file", func() {
				convey.So(err, convey.ShouldBeNil)
				convey.So(th.Name, convey.ShouldEqual, "renamed")
				_, statErr := os.Stat(filepath.Join(dir, "renamed.yml"))
				convey.So(statErr, convey.ShouldBeNil)
			})

			convey.Convey("It should remove the old file", func() {
				_, statErr := os.Stat(filepath.Join(dir, "original.yml"))
				convey.So(os.IsNotExist(statErr), convey.ShouldBeTrue)
			})
		})
	})
}

func TestList(t *testing.T) {
	convey.Convey("Given a themes directory", t, func() {
		dir := t.TempDir()
		origDir := ThemesDir

		ThemesDir = func() string { return dir }
		defer func() { ThemesDir = origDir }()

		convey.Convey("When no themes are saved", func() {
			names := List()

			convey.Convey("It should return just default", func() {
				convey.So(names, convey.ShouldResemble, []string{"default"})
			})
		})

		convey.Convey("When themes are saved", func() {
			th := Default()
			th.Name = "neon"
			th.Save()
			th.Name = "solarized"
			th.Save()

			names := List()

			convey.Convey("It should list all themes", func() {
				convey.So(len(names), convey.ShouldEqual, 3)
				convey.So(names[0], convey.ShouldEqual, "default")
			})
		})
	})
}

func BenchmarkThemeFgBrand(b *testing.B) {
	th := Default()
	SetActive(th)

	for index := 0; index < b.N; index++ {
		th.FgBrand()
	}
}

func BenchmarkThemeActive(b *testing.B) {
	th := Default()
	SetActive(th)

	for index := 0; index < b.N; index++ {
		Active()
	}
}
