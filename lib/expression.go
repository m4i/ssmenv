package lib

import (
	"encoding/json"
	"fmt"
	gopath "path"
	"strings"

	"github.com/aws/aws-sdk-go/service/ssm"
)

const secureMark = "@"

type expression struct {
	Name   string
	Value  string
	Secure bool
}

func parseExpression(expr string) (*expression, error) {
	sides := strings.SplitN(expr, "=", 2)
	if len(sides) != 2 {
		return nil, ErrInvalidExpression{Expr: expr}
	}
	lhs := sides[0]
	rhs := sides[1]

	secure := false
	if strings.HasSuffix(lhs, secureMark) {
		lhs = lhs[:len(lhs)-1]
		secure = true
	}

	value, err := unescape(rhs)
	if err != nil {
		return nil, err
	}

	return &expression{
		Name:   lhs,
		Value:  value,
		Secure: secure,
	}, nil
}

func newExpression(p *ssm.Parameter) *expression {
	return &expression{
		Name:   *p.Name,
		Value:  *p.Value,
		Secure: *p.Type == ssm.ParameterTypeSecureString,
	}
}

func (e *expression) serialize(path string) (string, error) {
	lhs, err := rel(e.Name, path)
	if err != nil {
		return "", err
	}
	return buildExpr("", lhs, e.Value, e.Secure)
}

func (e *expression) env() (string, error) {
	lhs := gopath.Base(e.Name)
	return buildExpr("", lhs, e.Value, false)
}

func (e *expression) export() (string, error) {
	lhs := exportableName(e.Name)
	return buildExpr("export ", lhs, e.Value, false)
}

func (e *expression) log() (string, error) {
	value := e.Value
	if e.Secure {
		value = strings.Repeat("*", 16)
	}
	return buildExpr("", abs(e.Name), value, e.Secure)
}

func (e *expression) parameter(path string) (*ssm.Parameter, error) {
	name, err := join(path, e.Name)
	if err != nil {
		return nil, err
	}

	_type := ssm.ParameterTypeString
	if e.Secure {
		_type = ssm.ParameterTypeSecureString
	}

	return &ssm.Parameter{
		Name:  &name,
		Type:  &_type,
		Value: &e.Value,
	}, nil
}

func buildExpr(prefix, lhs, value string, secure bool) (string, error) {
	rhs, err := escape(value)
	if err != nil {
		return "", err
	}

	mark := ""
	if secure {
		mark = secureMark
	}

	return fmt.Sprintf("%s%s%s=%s", prefix, lhs, mark, rhs), nil
}

func escape(value string) (string, error) {
	quoted := isQuoted(value)

	hasControl := false
	if !quoted {
		for i := 0; i < len(value); i++ {
			if value[i] < ' ' {
				hasControl = true
				break
			}
		}
	}

	if quoted || hasControl {
		bytes, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		value = string(bytes)
	}

	return value, nil
}

func unescape(value string) (string, error) {
	if isQuoted(value) {
		var unquoted string
		if err := json.Unmarshal([]byte(value), &unquoted); err != nil {
			return "", ErrUnmarshal{cause: err, value: value}
		}
		value = unquoted
	}
	return value, nil
}

func isQuoted(str string) bool {
	return strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`)
}

func exportableName(name string) string {
	name = gopath.Base(name)
	name = strings.Replace(name, ".", "_", -1)
	name = strings.Replace(name, "-", "_", -1)
	return name
}

func rel(name string, path string) (string, error) {
	absName := abs(name)

	if path == "" {
		return absName, nil
	}

	pathWithTrailingSlash := path
	if !strings.HasSuffix(pathWithTrailingSlash, "/") {
		pathWithTrailingSlash += "/"
	}
	if !strings.HasPrefix(absName, pathWithTrailingSlash) {
		return "", ErrPathMismatch{Path: path, Name: absName}
	}
	return absName[len(pathWithTrailingSlash):], nil
}
