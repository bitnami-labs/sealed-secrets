// +build go1.13

package multierror

import (
	"errors"
	"fmt"
)

// unwrap wraps go 1.13 Unwrap method
func unwrap(err error) error {
	return errors.Unwrap(err)
}

func errorSuffix(err error, format string, a ...interface{}) error {
	return fmt.Errorf("%w %s", err, fmt.Sprintf(format, a...))
}
