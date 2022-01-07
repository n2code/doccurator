package output

import (
	"path/filepath"

	"github.com/disiqueira/gotree/v3"
)

type VisualFileTree struct {
	tree gotree.Tree
	dirs map[string]gotree.Tree
}

func NewVisualFileTree(rootLabel string) VisualFileTree {
	return VisualFileTree{tree: gotree.New(rootLabel), dirs: make(map[string]gotree.Tree)}
}

func (t VisualFileTree) getDir(dirPath string) (dir gotree.Tree) {
	if dirPath == "." {
		return t.tree
	}
	dir = t.dirs[dirPath]
	if dir == nil {
		parentPath := filepath.Dir(dirPath)
		parentDir := t.getDir(parentPath)
		dir = parentDir.Add(filepath.Base(dirPath))
		t.dirs[dirPath] = dir
	}
	return
}

func (t VisualFileTree) InsertPath(filePath string, nodePrefix string) {
	file := filepath.Base(filePath)
	dir := t.getDir(filepath.Dir(filePath))
	dir.Add(nodePrefix + file)
}

func (t VisualFileTree) Render() string {
	return t.tree.Print()
}
