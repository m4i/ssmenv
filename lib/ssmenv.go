package lib

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	gopath "path"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// UseCommandInsteadOfExec is a flag to use exec.Command instead of syscall.Exec for testing.
var UseCommandInsteadOfExec = false

// Exec is the implementation of `ssmenv exec`.
func Exec(svc *ssm.SSM, paths []string, recursive bool, argv []string) error {
	if len(paths) == 0 {
		paths = append(paths, "")
	}
	if len(argv) == 0 {
		return ErrRequireCommand
	}

	argv0, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}

	params, err := getParametersByPaths(svc, paths, recursive)
	if err != nil {
		return err
	}

	paramsByName := make(map[string]*ssm.Parameter)
	for _, param := range params {
		paramsByName[gopath.Base(*param.Name)] = param
	}

	envs := os.Environ()
	for _, param := range paramsByName {
		env, err := newExpression(param).env() // nolint: vetshadow
		if err != nil {
			return err
		}
		envs = append(envs, env)
	}

	if !UseCommandInsteadOfExec {
		return syscall.Exec(argv0, argv, envs)
	}

	// for testing
	command := exec.Command(argv0, argv[1:]...)
	command.Env = envs
	out, err := command.CombinedOutput()
	fmt.Print(string(out))
	return err
}

// GetByPath is the implementation of `ssmenv get`.
func GetByPath(w io.Writer, svc *ssm.SSM, path string, recursive bool, exportFlag bool) error {
	params, err := GetParametersByPath(svc, path, recursive)
	if err != nil {
		return err
	}

	for _, param := range params {
		expr := newExpression(param)
		var line string
		var err error
		if exportFlag {
			line, err = expr.export()
		} else {
			line, err = expr.serialize(path)
		}
		if err != nil {
			return err
		}
		fmt.Fprintln(w, line)
	}

	return nil
}

// GetByName is the implementation of `ssmenv get NAME`.
func GetByName(w io.Writer, svc *ssm.SSM, path string, name string) error {
	name, err := join(path, name)
	if err != nil {
		return err
	}

	output, err := svc.GetParameter(&ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(w, *output.Parameter.Value)

	return nil
}

// Set is the implementation of `ssmenv set`.
func Set(w io.Writer, svc *ssm.SSM, path string, exprs []string) error {
	if len(exprs) < 1 {
		return ErrRequireNameAndValue
	}

	var params []*ssm.Parameter
	var names []*string
	for _, expr := range exprs {
		exprObj, err := parseExpression(expr)
		if err != nil {
			return err
		}
		param, err := exprObj.parameter(path)
		if err != nil {
			return err
		}
		params = append(params, param)
		names = append(names, param.Name)
	}

	return updateParameters(svc, params, names, []*string{}, w)
}

// Replace is the implementation of `ssmenv replace`.
func Replace(w io.Writer, svc *ssm.SSM, path string, recursive bool, exprs []string) error {
	if path == "" {
		return ErrRequirePath
	}
	if len(exprs) < 1 {
		return ErrRequireNameAndValue
	}

	var params []*ssm.Parameter
	for _, expr := range exprs {
		exprObj, err := parseExpression(expr)
		if err != nil {
			return err
		}
		if !recursive && !isBase(exprObj.Name) {
			return ErrSlashWithoutRecursive{Expr: expr}
		}
		param, err := exprObj.parameter(path)
		if err != nil {
			return err
		}
		params = append(params, param)
	}

	return ReplaceParameters(svc, path, recursive, params, w)
}
