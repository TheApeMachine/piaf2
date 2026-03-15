package core

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestLoadEmbedded(t *testing.T) {
	convey.Convey("Given embedded config", t, func() {
		config, err := LoadEmbedded(Embedded, "cfg/config.yml")

		convey.Convey("It should load without error", func() {
			convey.So(err, convey.ShouldBeNil)
			convey.So(config, convey.ShouldNotBeNil)
		})

		convey.Convey("It should have the research manager persona", func() {
			convey.So(config, convey.ShouldNotBeNil)
			convey.So(config.AI.Persona.Research.Manager, convey.ShouldContainSubstring, "A.I. research manager")
			convey.So(config.AI.Persona.Research.Manager, convey.ShouldContainSubstring, "consensus")
		})

		convey.Convey("It should have thinkcursion personas", func() {
			convey.So(len(config.AI.Thinkcursion.Personas), convey.ShouldBeGreaterThanOrEqualTo, 3)
			arch := config.AI.Thinkcursion.Personas["architect"]
			convey.So(arch.System, convey.ShouldContainSubstring, "architect")
			convey.So(arch.Model, convey.ShouldNotBeEmpty)
			convey.So(arch.BaseURL, convey.ShouldNotBeEmpty)
		})
	})
}
