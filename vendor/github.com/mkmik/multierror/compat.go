// +build !go1.13

package multierror

import "fmt"

// unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
//
// Taken from go1.13 errors.Unwrap.
// TODO: remove when we can stop caring about go <1.13.
func unwrap(err error) error {
	u, ok := err.(interface {
		Unwrap() error
	})
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func errorSuffix(err error, format string, a ...interface{}) error {
	return fmt.Errorf("%v %s", err, fmt.Sprintf(format, a...))
}
