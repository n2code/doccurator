package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (doc *Document) updateFromFile(location string) {
	doc.localStorage.updateFileStats(location)
	content, err := os.ReadFile(location)
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

func (stored *storedFile) updateFileStats(location string) {
	stat, err := os.Stat(location)
	if err != nil {
		panic(err)
	}
	stored.lastModified = unixTimestamp(stat.ModTime().Unix())
}

func (meta *contentMetadata) setFromContent(content []byte) {
	meta.size = int64(len(content))
	meta.sha256Hash = sha256.Sum256(content)
}

func (doc *Document) String() string {
	return fmt.Sprintf("Document %d\n  Path:     %s\n  Size:     %d bytes\n  SHA256:   %s\n  Recorded: %s\n  Modified: %s",
		doc.id,
		doc.localStorage.pathRelativeToLibrary(),
		doc.contentMetadata.size,
		hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		time.Unix(int64(doc.recorded), 0).Local().Format(time.RFC1123),
		time.Unix(int64(doc.localStorage.lastModified), 0).Local().Format(time.RFC1123))
}
