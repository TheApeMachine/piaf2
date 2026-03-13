package team

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestImplementationPlan(t *testing.T) {
	convey.Convey("Given an ImplementationPlan", t, func() {
		plan := &ImplementationPlan{
			Raw:            "raw text",
			FileTargets:    []string{"a.go", "b.go"},
			SuggestedOrder: []string{"a.go first"},
			Dependencies:   []string{"std lib"},
			Risks:          []string{"minimal"},
		}

		convey.Convey("Fields are populated", func() {
			convey.So(plan.Raw, convey.ShouldEqual, "raw text")
			convey.So(len(plan.FileTargets), convey.ShouldEqual, 2)
			convey.So(len(plan.SuggestedOrder), convey.ShouldEqual, 1)
			convey.So(len(plan.Dependencies), convey.ShouldEqual, 1)
			convey.So(len(plan.Risks), convey.ShouldEqual, 1)
		})
	})
}
