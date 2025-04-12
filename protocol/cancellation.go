package protocol

// CancelledNotification represents a notification that a request has been canceled
type CancelledNotification struct {
	RequestID RequestID `json:"requestId"`
	Reason    string    `json:"reason,omitempty"`
}

// NewCancelledNotification creates a new canceled notification
func NewCancelledNotification(requestID RequestID, reason string) *CancelledNotification {
	return &CancelledNotification{
		RequestID: requestID,
		Reason:    reason,
	}
}
