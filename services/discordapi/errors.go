package discordapi

import (
	"encoding/json"
	"strings"
)

// DiscordError struct
type DiscordError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Error struct
type Error struct {
	Message string `json:"message"`
	Err     error  `json:"error"`
	Code    int    `json:"code"`
}

// Error func
func (e *Error) Error() string {
	return e.Err.Error()
}

// ParseDiscordError func
func ParseDiscordError(err error) *Error {
	splitErr := strings.SplitN(err.Error(), ", ", 2)

	if len(splitErr) != 2 {
		return &Error{
			Code:    -1,
			Err:     err,
			Message: err.Error(),
		}
	}

	var discErr DiscordError
	umErr := json.Unmarshal([]byte(splitErr[1]), &discErr)

	if umErr != nil {
		return &Error{
			Code:    -1,
			Err:     umErr,
			Message: umErr.Error(),
		}
	}

	return &Error{
		Code:    discErr.Code,
		Err:     err,
		Message: discErr.Message,
	}
}
