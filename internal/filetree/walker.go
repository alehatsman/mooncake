// Package filetree provides directory tree walking functionality for with_filetree operations.
package filetree

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/alehatsman/mooncake/internal/pathutil"
)

// Item represents a single item in a file tree.
type Item struct {
	Src   string `yaml:"src"`
	Path  string `yaml:"path"`
	Name  string `yaml:"name"`
	State string `yaml:"state"`
	IsDir bool   `yaml:"is_dir"`
}

// Walker handles file tree operations.
type Walker struct {
	pathExpander *pathutil.PathExpander
}

// NewWalker creates a new file tree walker.
func NewWalker(pathExpander *pathutil.PathExpander) *Walker {
	return &Walker{
		pathExpander: pathExpander,
	}
}

// GetFileTree walks a directory tree and returns all files and directories.
func (w *Walker) GetFileTree(path string, currentDir string, context map[string]interface{}) ([]Item, error) {
	files := make([]Item, 0)

	root, err := w.pathExpander.ExpandPath(path, currentDir, context)
	if err != nil {
		return files, err
	}

	err = filepath.Walk(root, func(relativePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		state := "file"
		isDir := false
		if info.IsDir() {
			state = "directory"
			isDir = true
		}

		path := strings.Replace(relativePath, root, "", 1)
		name := info.Name()

		files = append(files, Item{
			Path:  path,
			Src:   relativePath,
			Name:  name,
			State: state,
			IsDir: isDir,
		})

		return nil
	})

	return files, err
}
