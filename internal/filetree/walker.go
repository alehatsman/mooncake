package filetree

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/alehatsman/mooncake/internal/pathutil"
)

// FileTreeItem represents a single item in a file tree
type FileTreeItem struct {
	Src   string `yaml:"src"`
	Path  string `yaml:"path"`
	State string `yaml:"state"`
}

// Walker handles file tree operations
type Walker struct {
	pathExpander *pathutil.PathExpander
}

// NewWalker creates a new file tree walker
func NewWalker(pathExpander *pathutil.PathExpander) *Walker {
	return &Walker{
		pathExpander: pathExpander,
	}
}

// GetFileTree walks a directory tree and returns all files and directories
func (w *Walker) GetFileTree(path string, currentDir string, context map[string]interface{}) ([]FileTreeItem, error) {
	files := make([]FileTreeItem, 0)

	root, err := w.pathExpander.ExpandPath(path, currentDir, context)
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

	return files, err
}
