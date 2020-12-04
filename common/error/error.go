package error


type AgentError struct {
	msg string
}

func (a *AgentError) Error() string {
	return a.msg
}

func New(msg string) *AgentError {
	return &AgentError{msg: msg}
}

func StateError() *AgentError {
	return New("[Agent状态异常] Agent处于一个不可操作的状态")
}