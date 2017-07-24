package lib

import (
	gopath "path"
	"regexp"
	"strings"
)

var (
	reName     = regexp.MustCompile(`^(?:/[-.\w]+){1,6}$|^[-.\w]+$`)
	rePath     = regexp.MustCompile(`^(?:/[-.\w]+){1,5}/?$|^/$`)
	reBase     = regexp.MustCompile(`^[-.\w]+$`)
	reRel      = regexp.MustCompile(`^[-.\w]+(?:/[-.\w]+)*$`)
	reReserved = regexp.MustCompile(`(?i)^/?(?:aws|ssm)`)
)

func isName(name string) bool {
	return reName.MatchString(name) && !reReserved.MatchString(name)
}

func isPath(path string) bool {
	return rePath.MatchString(path) && !reReserved.MatchString(path)
}

func isBase(base string) bool {
	return reBase.MatchString(base)
}

func isRel(rel string) bool {
	return reRel.MatchString(rel)
}

func validateName(name string) error {
	if !isName(name) {
		return ErrInvalidName{Name: name}
	}
	return nil
}

func validatePath(path string) error {
	if !isPath(path) {
		return ErrInvalidPath{Path: path}
	}
	return nil
}

func abs(name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	return "/" + name
}

func join(path string, name string) (string, error) {
	if path != "" {
		if err := validatePath(path); err != nil {
			return "", err
		}
		if !isRel(name) {
			return "", ErrAbsNameWithPath{Path: path, Name: name}
		}
		name = gopath.Join(path, name)
	}
	if err := validateName(name); err != nil {
		return "", err
	}
	return name, nil
}
