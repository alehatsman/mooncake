package utils

import (
	"os"
	"path"
	"path/filepath"
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

	if strings.HasPrefix(expandedPath, "../") {
		expandedPath = path.Join(currentDir, expandedPath)
		return expandedPath, nil
	}

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

type FileTreeItem struct {
	Src   string `yaml:"src"`
	Path  string `yaml:"path"`
	State string `yaml:"state"`
}

func GetFileTree(path string, currentDir string, context map[string]interface{}) ([]FileTreeItem, error) {
	files := make([]FileTreeItem, 0)

	root, err := ExpandPath(path, currentDir, context)
	if err != nil {
		return files, err
	}

	err = filepath.Walk(root, func(relativePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		state := "file"
		if info.IsDir() {
			state = "directory"
		}

		path := strings.Replace(relativePath, root, "", 1)

		files = append(files, FileTreeItem{
			Path:  path,
			Src:   relativePath,
			State: state,
		})

		return nil
	})

	return files, nil
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
	pongo2.RegisterFilter("expanduser", func(in *pongo2.Value, param *pongo2.Value) (*pongo2.Value, *pongo2.Error) {
		expandedPath, err := ExpandPath(in.String(), "", variables)

		if err != nil {
			return nil, &pongo2.Error{
				Sender:    "filter:expanduser",
				OrigError: err,
			}
		}

		return pongo2.AsValue(expandedPath), nil
	})

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
