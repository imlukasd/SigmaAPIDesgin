package auth

import "fmt"

type errNotImplemented string

func (e errNotImplemented) Error() string {
	return fmt.Sprintf("auth %s not implemented", string(e))
}
