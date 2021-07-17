package internal

import (
	"crypto/sha256"
	"os"
	"path/filepath"
)

func (doc *Document) UpdateFromFile() {
	doc.localStorage.updateFileStats()
	content, err := os.ReadFile(doc.localStorage.pathRelativeToLibrary())
	if err != nil {
		panic(err)
	}
	doc.contentMetadata.setFromContent(content)
}

func (stored *storedFile) setFromRelativePath(relativePath string) {
	stored.directory = filepath.ToSlash(filepath.Dir(relativePath))
	stored.name = filepath.Base(relativePath)
}

func (stored *storedFile) pathRelativeToLibrary() string {
	return filepath.Join(filepath.FromSlash(stored.directory), stored.name)
}

func (stored *storedFile) updateFileStats() {
	stat, err := os.Stat(stored.pathRelativeToLibrary())
	if err != nil {
		panic(err)
	}
	stored.lastModified = unixTimestamp(stat.ModTime().Unix())
}

func (meta *contentMetadata) setFromContent(content []byte) {
	meta.size = int64(len(content))
	meta.sha256Hash = sha256.Sum256(content)
}
