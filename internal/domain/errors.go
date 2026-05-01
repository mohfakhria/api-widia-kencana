package domain

type Error struct {
	Kind    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e.Kind == t.Kind
}

var (
	ErrNotFound        = &Error{Kind: "not_found"}
	ErrUnauthorized    = &Error{Kind: "unauthorized"}
	ErrForbidden       = &Error{Kind: "forbidden"}
	ErrInvalidInput    = &Error{Kind: "invalid_input"}
	ErrUnavailable     = &Error{Kind: "unavailable"}
	ErrInternalFailure = &Error{Kind: "internal_failure"}
)

func NewError(kind *Error, message string) error {
	return &Error{
		Kind:    kind.Kind,
		Message: message,
	}
}
