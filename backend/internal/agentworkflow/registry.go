package agentworkflow

// Registry stores the workflow definition for each supported action.
type Registry []Action

// NewDefaultRegistry defines the available actions and whether they require
// candidate approval. Input and result verification can be attached to each
// action independently.
func NewDefaultRegistry() Registry {
	return Registry{
		{
			Name:             "FileRead",
			RequiresApproval: false,
			Trigger:          ToolNamed("FileRead"),
		},
		{
			Name:             "FileEdit",
			RequiresApproval: true,
			Trigger:          ToolNamed("FileEdit"),
			VerifyInput:      VerifyFileEditInput,
		},
		{
			Name:             "RunTests",
			RequiresApproval: true,
			Trigger:          ToolNamed("RunTests"),
		},
		{
			Name:             "Grep",
			RequiresApproval: false,
			Trigger:          ToolNamed("Grep"),
		},
		{
			Name:             "Glob",
			RequiresApproval: false,
			Trigger:          ToolNamed("Glob"),
		},
		{
			Name:             "RunCommand",
			RequiresApproval: true,
			Trigger:          ToolNamed("RunCommand"),
		},
	}
}

// Resolve returns the first action whose trigger matches the request.
func (r Registry) Resolve(request Request) (Action, bool) {
	for _, action := range r {
		if action.ShouldRun(request) {
			return action, true
		}
	}
	return Action{}, false
}
