package multierror

import (
	"bytes"
	"fmt"
	"strings"
)

// Error bundles multiple errors and make them obey the error interface
type Error struct {
	errs      []error
	formatter Formatter
}

// Formatter allows to customize the rendering of the multierror.
type Formatter func(errs []string) string

var DefaultFormatter = func(errs []string) string {
	buf := bytes.NewBuffer(nil)

	fmt.Fprintf(buf, "%d errors occurred:", len(errs))
	for _, line := range errs {
		fmt.Fprintf(buf, "\n%s", line)
	}

	return buf.String()
}

func (e *Error) Error() string {
	var f Formatter = DefaultFormatter
	if e.formatter != nil {
		f = e.formatter
	}

	var lines []string
	for _, err := range e.errs {
		lines = append(lines, err.Error())
	}

	return f(lines)
}

type JoinOption func(*joinOptions)
type joinOptions struct {
	formatter   Formatter
	transformer func([]error) []error
}

func WithFormatter(f Formatter) JoinOption {
	return func(o *joinOptions) { o.formatter = f }
}

func WithTransformer(t func([]error) []error) JoinOption {
	return func(o *joinOptions) { o.transformer = t }
}

// Join turns a slice of errors into a multierror.
func Join(errs []error, opts ...JoinOption) error {
	var o joinOptions
	for _, opt := range opts {
		opt(&o)
	}
	if o.transformer != nil {
	errs = o.transformer(errs)
	}
	return &Error{errs: errs, formatter: o.formatter}
}

// Fold is deprecated, use Join instead.
//
// Fold turns a slice of errors into a multierror.
func Fold(errs []error) error {
	return Join(errs)
}

// Split returns the underlying list of errors wrapped in a multierror.
// If err is not a multierror, then a singleton list is returned.
func Split(err error) []error {
	if me, ok := err.(*Error); ok {
		return me.errs
	} else {
		return []error{err}
	}
}

// Unfold is deprecated, use Split instead.
//
// Unfold returns the underlying list of errors wrapped in a multierror.
// If err is not a multierror, then a singleton list is returned.
func Unfold(err error) []error {
	return Split(err)
}

// Append creates a new mutlierror.Error structure or appends the arguments to an existing multierror
// err can be nil, or can be a non-multierror error.
//
// If err is nil and errs has only one element, that element is returned.
// I.e. a singleton error is never treated and (thus rendered) as a multierror.
// This also also effectively allows users to just pipe through the error value of a function call,
// without having to first check whether the error is non-nil.
func Append(err error, errs ...error) error {
	if err == nil && len(errs) == 1 {
		return errs[0]
	}
	if len(errs) == 1 && errs[0] == nil {
		return err
	}
	if err == nil {
		return Fold(errs)
	}
	switch err := err.(type) {
	case *Error:
		err.errs = append(err.errs, errs...)
		return err
	default:
		return Fold(append([]error{err}, errs...))
	}
}

// Uniq deduplicates a list of errors
func Uniq(errs []error) []error {
	type groupingKey struct {
		msg    string
		tagged bool
	}
	var ordered []groupingKey
	grouped := map[groupingKey][]error{}

	for _, err := range errs {
		msg, tag := TaggedError(err)
		key := groupingKey{
			msg:    msg,
			tagged: tag != "",
		}
		if _, ok := grouped[key]; !ok {
			ordered = append(ordered, key)
		}
		grouped[key] = append(grouped[key], err)
	}

	var res []error
	for _, key := range ordered {
		group := grouped[key]
		err := group[0]
		if key.tagged {
			var tags []string
			for _, e := range group {
				_, tag := TaggedError(e)
				tags = append(tags, tag)
			}
			err = errorSuffix(unwrap(err), "(%s)", strings.Join(tags, ", "))
		} else {
			if n := len(group); n > 1 {
				err = errorSuffix(err, "repeated %d times", n)
			}
		}
		res = append(res, err)
	}

	return res
}

type TaggableError interface {
	// TaggedError is like Error() but splits the error from the tag.
	TaggedError() (string, string)
}

// TaggedError is like Error() but if err implements TaggedError, it will
// invoke TaggeddError() and return error message and the tag. Otherwise the tag will be empty.
func TaggedError(err error) (string, string) {
	if te, ok := err.(TaggableError); ok {
		return te.TaggedError()
	}
	return err.Error(), ""
}

type taggedError struct {
	tag string
	err error
}

// Tag wraps an error with a tag. The resulting error implements the TaggableError interface
// and thus the tags can be unwrapped by Uniq in order to deduplicate error messages without loosing
// context.
func Tag(tag string, err error) error {
	return taggedError{tag: tag, err: err}
}

func (t taggedError) Error() string {
	return fmt.Sprintf("%s (%s)", t.err.Error(), t.tag)
}

func (t taggedError) Unwrap() error {
	return t.err
}

func (t taggedError) TaggedError() (string, string) {
	return t.err.Error(), t.tag
}

// Format sets a custom formatter if err is a multierror.
func Format(err error, f Formatter) error {
	if me, ok := err.(*Error); ok {
		cpy := *me
		cpy.formatter = f
		return &cpy
	} else {
		return err
	}
}

// InlineFormatter formats all errors in
func InlineFormatter(errs []string) string {
	return strings.Join(errs, "; ")
}

// Transform applies a transformer to an unfolded multierror and re-wraps the result.
func Transform(err error, fn func([]error) []error) error {
	return Fold(fn(Unfold(err)))
}
