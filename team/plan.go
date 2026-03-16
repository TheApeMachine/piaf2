package team

/*
ImplementationPlan stores file-level changes, dependencies, ordering, and risks.
Produced by Architect from the kanban. DeveloperCount and Assignments come from
parallelizability analysis.
*/
type ImplementationPlan struct {
	Raw             string
	FileTargets     []string
	SuggestedOrder  []string
	Dependencies    []string
	Risks           []string
	DeveloperCount  int
	TaskAssignments map[int]string
}
