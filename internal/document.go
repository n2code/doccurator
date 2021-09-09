package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func NewDocument(id DocumentId) *Document {
	now := unixTimestamp(time.Now().Unix())
	return &Document{
		id:       id,
		recorded: now,
		changed:  now,
	}
}

func (doc *Document) setPath(relativePath string) {
	doc.localStorage.setFromRelativePath(relativePath)
	doc.updateChangeDate()
}

func (doc *Document) updateChangeDate() {
	doc.changed = unixTimestamp(time.Now().Unix())
}

func (doc *Document) updateFromFile(location string) {
	statsChanged := doc.localStorage.updateFileStats(location)
	content, err := os.ReadFile(location)
	if err != nil {
		panic(err)
	}
	contentChanged := doc.contentMetadata.setFromContent(content)
	if statsChanged || contentChanged {
		doc.updateChangeDate()
	}
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

func (doc *Document) String() string {
	return fmt.Sprintf("Document %d\n  Path:     %s\n  Size:     %d bytes\n  SHA256:   %s\n  Recorded: %s\n  Modified: %s",
		doc.id,
		doc.localStorage.pathRelativeToLibrary(),
		doc.contentMetadata.size,
		hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		time.Unix(int64(doc.recorded), 0).Local().Format(time.RFC1123),
		time.Unix(int64(doc.localStorage.lastModified), 0).Local().Format(time.RFC1123))
}
