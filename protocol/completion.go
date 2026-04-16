package protocol

// CompleteRequest represents a request for completion options
type CompleteRequest struct {
	Argument struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"argument"`
	Ref interface{} `json:"ref"` // Can be PromptReference or ResourceReference
}

// Reference types
type PromptReference struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type ResourceReference struct {
	Type string `json:"type"`
	URI  string `json:"uri"`
}

// CompleteResult represents the response to a completion request
type CompleteResult struct {
	Completion *Complete `json:"completion"`
}

type Complete struct {
	Values  []string `json:"values"`
	HasMore bool     `json:"hasMore,omitempty"`
	Total   int      `json:"total,omitempty"`
}

// NewCompleteRequest creates a new completion request
func NewCompleteRequest(argName string, argValue string, ref interface{}) *CompleteRequest {
	return &CompleteRequest{
		Argument: struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{
			Name:  argName,
			Value: argValue,
		},
		Ref: ref,
	}
}

// NewCompleteResult creates a new completion response
func NewCompleteResult(values []string, hasMore bool, total int) *CompleteResult {
	return &CompleteResult{
		Completion: &Complete{
			Values:  values,
			HasMore: hasMore,
			Total:   total,
		},
	}
}
