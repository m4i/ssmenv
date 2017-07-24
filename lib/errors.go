package lib

import (
	"errors"
	"fmt"
)

// Errors with a fixed message.
var (
	ErrRequireCommand      = errors.New("command is required")
	ErrRequireNameAndValue = errors.New("name=value is required")
	ErrRequirePath         = errors.New("path is required")
)

// ErrSlashWithoutRecursive describes that a name contains slashes without a recrusive flag.
type ErrSlashWithoutRecursive struct {
	Expr string
}

func (e ErrSlashWithoutRecursive) Error() string {
	return fmt.Sprintf("a name must not contain slashes without a recrusive flag: %v", e.Expr)
}

// ErrInvalidName records an error for an invalid name.
type ErrInvalidName struct {
	Name string
}

func (e ErrInvalidName) Error() string {
	return fmt.Sprintf("invalid name: %v", e.Name)
}

// ErrInvalidPath records an error for an invalid path.
type ErrInvalidPath struct {
	Path string
}

func (e ErrInvalidPath) Error() string {
	return fmt.Sprintf("invalid path: %v", e.Path)
}

// ErrAbsNameWithPath describes an absolute name are given with a path.
type ErrAbsNameWithPath struct {
	Path, Name string
}

func (e ErrAbsNameWithPath) Error() string {
	return fmt.Sprintf("an absolute name can not be given with a path: path=%#v, name=%#v", e.Path, e.Name)
}

// ErrPathMismatch describes that a name does not begin with a path.
type ErrPathMismatch struct {
	Path, Name string
}

func (e ErrPathMismatch) Error() string {
	return fmt.Sprintf("a name must begin with a path: path=%#v, name=%#v", e.Path, e.Name)
}

// ErrInvalidExpression describes a problem parsing a expression.
type ErrInvalidExpression struct {
	Expr string
}

func (e ErrInvalidExpression) Error() string {
	return fmt.Sprintf(`a expression must be "name[@]=value": %#v`, e.Expr)
}

// ErrUnmarshal describes a problem parsing a expression.
type ErrUnmarshal struct {
	cause error
	value string
}

func (e ErrUnmarshal) Error() string {
	return fmt.Sprintf("invalid value %#v: %s", e.value, e.cause.Error())
}
