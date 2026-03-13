package team

/*
ImplementationPlan stores file-level changes, dependencies, ordering, and risks.
Produced by Architect from the kanban. Raw holds the full Architect response for context.
*/
type ImplementationPlan struct {
	Raw            string
	FileTargets    []string
	SuggestedOrder []string
	Dependencies   []string
	Risks          []string
}
