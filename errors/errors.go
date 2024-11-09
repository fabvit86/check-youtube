package errors

import "fmt"

type GetSessionErr struct {
	Err error
}

func (e GetSessionErr) Error() string {
	return fmt.Sprintf("failed to get session: %s", e.Err.Error())
}

type SaveSessionErr struct {
	Err error
}

func (e SaveSessionErr) Error() string {
	return fmt.Sprintf("failed to save session: %s", e.Err.Error())
}

type ValueNotFoundInCtx struct {
	Key fmt.Stringer
}

func (e ValueNotFoundInCtx) Error() string {
	return fmt.Sprintf("%s not found in context", e.Key.String())
}
