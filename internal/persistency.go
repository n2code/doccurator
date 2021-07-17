package internal

import (
	"encoding/base64"
	"encoding/json"
)

func (doc *Document) MarshalJSON() ([]byte, error) {
	persistedDoc := struct {
		Dir      string
		File     string
		Size     int64
		Sha256   string
		Recorded unixTimestamp
		Modified unixTimestamp
	}{
		Dir:      doc.localStorage.directory,
		File:     doc.localStorage.name,
		Size:     doc.contentMetadata.size,
		Sha256:   base64.StdEncoding.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		Recorded: doc.recorded,
		Modified: doc.localStorage.lastModified,
	}
	return json.Marshal(persistedDoc)
}

func (lib *library) MarshalJSON() ([]byte, error) {
	root := struct {
		Documents map[DocumentId]*Document
	}{
		Documents: lib.documents,
	}
	return json.Marshal(root)
}
