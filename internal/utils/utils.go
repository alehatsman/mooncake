package utils

import (
	"os"
	"path"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/flosch/pongo2/v6"
)

func ExpandPath(originalPath string, currentDir string, context map[string]interface{}) (string, error) {
	expandedPath, err := Render(originalPath, context)
	if err != nil {
		return "", nil
	}

	expandedPath = strings.Trim(expandedPath, " ")

	if strings.HasPrefix(expandedPath, ".") {
		expandedPath = path.Join(currentDir, expandedPath[1:])
	}

	if strings.HasPrefix(expandedPath, "~/") {
		home := os.Getenv("HOME")
		expandedPath = home + expandedPath[1:]
	}

	return expandedPath, nil
}

func GetDirectoryOfFile(path string) string {
	return path[0:strings.LastIndex(path, "/")]
}

func Evaluate(expression string, variables map[string]interface{}) (interface{}, error) {
	evaluableExpression, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return nil, err
	}

	evalResult, err := evaluableExpression.Evaluate(variables)
	if err != nil {
		return nil, err
	}

	return evalResult, nil
}

func Render(template string, variables map[string]interface{}) (string, error) {
	pongoTemplate, err := pongo2.FromString(template)

	if err != nil {
		return "", err
	}

	output, err := pongoTemplate.Execute(variables)

	if err != nil {
		return "", err
	}

	return output, nil
}
