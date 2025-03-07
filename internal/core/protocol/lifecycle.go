package protocol

// ShutdownRequest is sent to gracefully shut down the connection
type ShutdownRequest struct {
}

// ShutdownResult confirms the shutdown request
type ShutdownResult struct {
}

// ExitNotification is sent to terminate the connection
type ExitNotification struct {
}

// CancelRequest is sent to cancel a pending request
type CancelRequest struct {
	// ID of the request to cancel
	ID int `json:"id"`
}

// CancelResult confirms the cancellation
type CancelResult struct {
}

// ProgressToken identifies a specific ongoing operation
type ProgressToken string

// ProgressParams contains information about operation progress
type ProgressParams struct {
	// Token identifies the operation
	Token ProgressToken `json:"token"`
	// Value indicates the progress (0-100%)
	Value int `json:"value"`
	// Message provides additional progress information
	Message string `json:"message,omitempty"`
}

// ProgressNotification reports progress for long-running operations
type ProgressNotification struct {
	// Params contain the progress information
	Params ProgressParams `json:"params"`
}
