package cmd

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/piaf/core"
)

func TestSortedPersonaNames(t *testing.T) {
	convey.Convey("Given a map of personas", t, func() {
		personas := map[string]core.PersonaConfig{
			"philosopher":   {System: "p", Model: "m", BaseURL: "u"},
			"ai_researcher": {System: "a", Model: "m", BaseURL: "u"},
			"mathematician": {System: "m", Model: "m", BaseURL: "u"},
		}

		convey.Convey("It should return names in sorted order", func() {
			names := sortedPersonaNames(personas)
			convey.So(names, convey.ShouldResemble, []string{
				"ai_researcher", "mathematician", "philosopher",
			})
		})
	})
}

func TestBuildAgents(t *testing.T) {
	convey.Convey("Given persona configs and sorted names", t, func() {
		personas := map[string]core.PersonaConfig{
			"alpha": {
				System:  "You are alpha.",
				Model:   "gpt-oss-120b",
				BaseURL: "http://localhost:11434/v1",
			},
			"beta": {
				System:  "You are beta.",
				Model:   "gpt-oss-120b",
				BaseURL: "http://localhost:11434/v1",
			},
		}
		names := []string{"alpha", "beta"}

		convey.Convey("It should create one agent per persona", func() {
			agents := buildAgents(names, personas)
			convey.So(len(agents), convey.ShouldEqual, 2)
		})

		convey.Convey("It should preserve name and system prompt", func() {
			agents := buildAgents(names, personas)
			convey.So(agents[0].name, convey.ShouldEqual, "alpha")
			convey.So(agents[0].system, convey.ShouldEqual, "You are alpha.")
			convey.So(agents[1].name, convey.ShouldEqual, "beta")
			convey.So(agents[1].system, convey.ShouldEqual, "You are beta.")
		})

		convey.Convey("It should wrap each provider with retry", func() {
			agents := buildAgents(names, personas)
			convey.So(agents[0].provider.Name(), convey.ShouldEqual, "OpenAI")
		})
	})
}

func TestExpandHome(t *testing.T) {
	convey.Convey("Given paths with and without tilde", t, func() {
		convey.Convey("It should expand tilde to home directory", func() {
			result := expandHome("~/foo/bar")
			convey.So(result, convey.ShouldNotStartWith, "~")
			convey.So(result, convey.ShouldContainSubstring, "foo/bar")
		})

		convey.Convey("It should leave absolute paths untouched", func() {
			result := expandHome("/tmp/output.md")
			convey.So(result, convey.ShouldEqual, "/tmp/output.md")
		})

		convey.Convey("It should leave relative paths untouched", func() {
			result := expandHome("output.md")
			convey.So(result, convey.ShouldEqual, "output.md")
		})
	})
}

func BenchmarkSortedPersonaNames(b *testing.B) {
	personas := map[string]core.PersonaConfig{
		"philosopher":   {System: "p"},
		"ai_researcher": {System: "a"},
		"mathematician": {System: "m"},
		"physicist":     {System: "ph"},
		"biologist":     {System: "b"},
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sortedPersonaNames(personas)
	}
}

func BenchmarkBuildAgents(b *testing.B) {
	personas := map[string]core.PersonaConfig{
		"alpha": {System: "a", Model: "m", BaseURL: "http://localhost:11434/v1"},
		"beta":  {System: "b", Model: "m", BaseURL: "http://localhost:11434/v1"},
		"gamma": {System: "g", Model: "m", BaseURL: "http://localhost:11434/v1"},
	}
	names := []string{"alpha", "beta", "gamma"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buildAgents(names, personas)
	}
}
