package document

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"time"
)

func (doc *document) updateChangeDate() {
	doc.changed = unixTimestamp(time.Now().Unix())
}

func (stored *storedFile) setFromRelativePath(relativePath string) {
	stored.directory = filepath.ToSlash(filepath.Dir(relativePath))
	stored.name = filepath.Base(relativePath)
}

func (stored *storedFile) pathRelativeToLibrary() string {
	return filepath.Join(filepath.FromSlash(stored.directory), stored.name)
}

func (stored *storedFile) updateFileStats(location string) (hasChanged bool) {
	stat, err := os.Stat(location)
	if err != nil {
		panic(err)
	}
	oldLastModified := stored.lastModified
	stored.lastModified = unixTimestamp(stat.ModTime().Unix())
	if stored.lastModified != oldLastModified {
		hasChanged = true
	}
	return
}

func (meta *contentMetadata) setFromContent(content []byte) (hasChanged bool) {
	oldSize := meta.size
	oldHash := meta.sha256Hash
	meta.size = int64(len(content))
	meta.sha256Hash = sha256.Sum256(content)
	if meta.size != oldSize || meta.sha256Hash != oldHash {
		hasChanged = true
	}
	return
}
