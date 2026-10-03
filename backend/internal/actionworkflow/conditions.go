package actionworkflow

// ToolNamed triggers an action when the model requests the matching tool.
func ToolNamed(name string) TriggerCondition {
	return func(request Request) bool {
		return request.Name == name
	}
}
