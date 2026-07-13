package tunnel

type ErrorType string

const (
	ErrorTypeHostUnavailable        ErrorType = "HostUnavailableError"
	ErrorTypeClientAlreadyConnected ErrorType = "ClientAlreadyConnectedError"
	ErrorTypeHandshakeFailed        ErrorType = "HandshakeFailedError"
	ErrorTypeCommandAlreadyRunning  ErrorType = "CommandAlreadyRunningError"
	ErrorTypeMaxOutputExceeded      ErrorType = "MaxOutputExceededError"
	ErrorTypeCommandStartFailed     ErrorType = "CommandStartFailedError"
	ErrorTypeIdleSessionTimeout     ErrorType = "IdleSessionTimeoutError"
	ErrorTypeProtocol               ErrorType = "ProtocolError"
)
