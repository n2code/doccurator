package document

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func (id DocumentId) String() string {
	return fmt.Sprintf("%d", id)
}

type jsonDoc struct {
	Dir          string
	File         string
	Size         int64
	Sha256       string
	Recorded     unixTimestamp
	Changed      unixTimestamp
	FileModified unixTimestamp
}

func (doc *Document) MarshalJSON() ([]byte, error) {
	persistedDoc := jsonDoc{
		Dir:          doc.localStorage.directory,
		File:         doc.localStorage.name,
		Size:         doc.contentMetadata.size,
		Sha256:       hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		Recorded:     doc.recorded,
		Changed:      doc.changed,
		FileModified: doc.localStorage.lastModified,
	}
	return json.Marshal(persistedDoc)
}

func (doc *Document) UnmarshalJSON(blob []byte) error {
	var loadedDoc jsonDoc
	err := json.Unmarshal(blob, &loadedDoc)
	if err != nil {
		return err
	}
	doc.id = MissingId
	doc.localStorage.directory = loadedDoc.Dir
	doc.localStorage.name = loadedDoc.File
	doc.localStorage.lastModified = loadedDoc.FileModified
	doc.contentMetadata.size = loadedDoc.Size
	shaBytes, err := hex.DecodeString(loadedDoc.Sha256)
	if err != nil {
		panic(err)
	}
	if len(shaBytes) != 32 {
		panic("persisted hash has bad length")
	}
	copy(doc.contentMetadata.sha256Hash[:], shaBytes)
	doc.recorded = loadedDoc.Recorded
	doc.changed = loadedDoc.Changed
	return nil
}
