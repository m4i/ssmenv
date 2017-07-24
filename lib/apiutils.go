package lib

import (
	"fmt"
	"io"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/m4i/ssmenv/semaphore"
)

const maxNames = 10

// MaxConnection is the max number of concurrent connections to the AWS API.
var MaxConnection = 4

func describeParameters(svc *ssm.SSM, paths []string, recursive bool) ([]*ssm.ParameterMetadata, error) {
	for i, path := range paths {
		if path == "" {
			path = "/"
			paths[i] = path
		}
		if err := validatePath(path); err != nil {
			return nil, err
		}
	}

	option := "OneLevel"
	if recursive {
		option = "Recursive"
	}
	input := ssm.DescribeParametersInput{
		MaxResults: aws.Int64(50),
		ParameterFilters: []*ssm.ParameterStringFilter{{
			Key:    aws.String("Path"),
			Option: &option,
			Values: aws.StringSlice(paths),
		}},
	}
	var metas []*ssm.ParameterMetadata
	fn := func(output *ssm.DescribeParametersOutput, _ bool) bool {
		metas = append(metas, output.Parameters...)
		return true
	}
	if err := svc.DescribeParametersPages(&input, fn); err != nil {
		return nil, err
	}
	return metas, nil
}

func getParametersByPaths(svc *ssm.SSM, paths []string, recursive bool) ([]*ssm.Parameter, error) {
	paramsSlice := make([][]*ssm.Parameter, len(paths))
	sem := semaphore.New(MaxConnection)
	for i, path := range paths {
		i, path := i, path
		sem.Go(func() error {
			params, err := GetParametersByPath(svc, path, recursive)
			if err != nil {
				return err
			}
			paramsSlice[i] = params
			return nil
		})
	}
	if err := sem.Wait(); err != nil {
		return nil, err
	}

	var params []*ssm.Parameter
	for _, ps := range paramsSlice {
		params = append(params, ps...)
	}
	return params, nil
}

// GetParametersByPath is a wrapper of SSM.GetParametersByPath()
func GetParametersByPath(svc *ssm.SSM, path string, recursive bool) ([]*ssm.Parameter, error) {
	if path == "" {
		path = "/"
	}
	if err := validatePath(path); err != nil {
		return nil, err
	}

	input := ssm.GetParametersByPathInput{
		Path:           &path,
		Recursive:      &recursive,
		WithDecryption: aws.Bool(true),
	}
	var params []*ssm.Parameter
	fn := func(output *ssm.GetParametersByPathOutput, _ bool) bool {
		params = append(params, output.Parameters...)
		return true
	}
	if err := svc.GetParametersByPathPages(&input, fn); err != nil {
		return nil, err
	}
	return params, nil
}

// GetParametersByNames is a wrapper of SSM.GetParameters()
func GetParametersByNames(svc *ssm.SSM, names []*string) ([]*ssm.Parameter, error) {
	for _, name := range names {
		if err := validateName(*name); err != nil {
			return nil, err
		}
	}

	var params []*ssm.Parameter

	sem := semaphore.New(MaxConnection)
	var mu sync.Mutex
	for i := 0; i < len(names); i += maxNames {
		ns := names[i:]
		if len(ns) > maxNames {
			ns = ns[:maxNames]
		}
		sem.Go(func() error {
			output, err := svc.GetParameters(&ssm.GetParametersInput{
				Names:          ns,
				WithDecryption: aws.Bool(true),
			})
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			params = append(params, output.Parameters...)
			return nil
		})
	}
	if err := sem.Wait(); err != nil {
		return nil, err
	}

	return params, nil
}

// nolint: gocyclo
func updateParameters(
	svc *ssm.SSM,
	params []*ssm.Parameter,
	names []*string,
	deleteNames []*string,
	log io.Writer,
) error {
	var println func(string)
	if log == nil {
		println = func(string) {}
	} else {
		var mu sync.Mutex
		println = func(mes string) {
			mu.Lock()
			defer mu.Unlock()
			fmt.Fprintln(log, mes)
		}
	}

	oldParams, err := GetParametersByNames(svc, names)
	if err != nil {
		return err
	}
	oldParamsByName := make(map[string]*ssm.Parameter)
	for _, oldParam := range oldParams {
		oldParamsByName[abs(*oldParam.Name)] = oldParam
	}

	sem := semaphore.New(MaxConnection)

	for _, param := range params {
		expr, err := newExpression(param).log()
		if err != nil {
			return err
		}

		if old, ok := oldParamsByName[abs(*param.Name)]; ok && *param.Type == *old.Type && *param.Value == *old.Value {
			println("UNCHANGED " + expr)
			continue
		}

		if err := validateName(*param.Name); err != nil {
			return err
		}

		param := param
		sem.Go(func() error {
			_, err := svc.PutParameter(&ssm.PutParameterInput{
				Name:      param.Name,
				Value:     param.Value,
				Type:      param.Type,
				Overwrite: aws.Bool(true),
			})
			if err != nil {
				return err
			}
			println("PUT " + expr)
			return nil
		})
	}

	for _, name := range deleteNames {
		if err := validateName(*name); err != nil {
			return err
		}

		name := name
		sem.Go(func() error {
			_, err := svc.DeleteParameter(&ssm.DeleteParameterInput{Name: name})
			if err != nil {
				return err
			}
			println("DELETE " + abs(*name))
			return nil
		})
	}

	return sem.Wait()
}

// ReplaceParameters replaces all the parameters of the given path.
func ReplaceParameters(svc *ssm.SSM, path string, recursive bool, params []*ssm.Parameter, log io.Writer) error {
	oldMetas, err := describeParameters(svc, []string{path}, recursive)
	if err != nil {
		return err
	}

	nameExists := make(map[string]bool)
	for _, param := range params {
		nameExists[abs(*param.Name)] = true
	}

	var names []*string
	var deleteNames []*string
	for _, oldMeta := range oldMetas {
		if nameExists[abs(*oldMeta.Name)] {
			names = append(names, oldMeta.Name)
		} else {
			deleteNames = append(deleteNames, oldMeta.Name)
		}
	}

	return updateParameters(svc, params, names, deleteNames, log)
}
