package agentworkflow

type Input map[string]any

type ValidationContext interface {
	Get(path string) (content string, exists bool)
}

type Request struct {
	Name  string
	Input Input
}

type TriggerCondition func(Request) bool

type InputVerifier func(
	input Input,
	context ValidationContext,
) (Input, error)

type ResultVerifier func(
	output string,
	isError bool,
) error

type Action struct {
	Name             string
	RequiresApproval bool
	Trigger          TriggerCondition
	VerifyInput      InputVerifier
	VerifyResult     ResultVerifier
}

func (a Action) ShouldRun(request Request) bool {
	if a.Trigger == nil {
		return false
	}
	return a.Trigger(request)
}

func (a Action) ValidateInput(
	input Input,
	context ValidationContext,
) (Input, error) {
	if a.VerifyInput == nil {
		return input, nil
	}
	return a.VerifyInput(input, context)
}

func (a Action) ValidateResult(output string, isError bool) error {
	if a.VerifyResult == nil {
		return nil
	}
	return a.VerifyResult(output, isError)
}
