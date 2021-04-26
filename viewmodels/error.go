package viewmodels

// ErrorResponse response struct for an error
type ErrorResponse struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(status int, err string, message string) ErrorResponse {
	return ErrorResponse{
		Error:   err,
		Message: message,
	}
}
