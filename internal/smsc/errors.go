package smsc

import "fmt"

var (
	ErrInvalidPDU      = &SmppError{Code: "invalid_pdu", Message: "invalid PDU format"}
	ErrInvalidSystemID = &SmppError{Code: "invalid_system_id", Message: "invalid system ID"}
	ErrInvalidPassword = &SmppError{Code: "invalid_password", Message: "invalid password"}
	ErrSessionClosed   = &SmppError{Code: "session_closed", Message: "session is closed"}
	ErrTimeout         = &SmppError{Code: "timeout", Message: "operation timed out"}
)

type SmppError struct {
	Code    string
	Message string
}

func (e *SmppError) Error() string {
	return fmt.Sprintf("SMPP error [%s]: %s", e.Code, e.Message)
}

func NewSmppError(code, message string) error {
	return &SmppError{
		Code:    code,
		Message: message,
	}
}
