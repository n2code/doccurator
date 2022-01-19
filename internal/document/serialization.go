package document

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/n2code/ndocid"
)

func (id DocumentId) String() string {
	return fmt.Sprintf("%s", ndocid.EncodeUint64(uint64(id)))
}

func (doc *document) String() string {
	return fmt.Sprintf(`Document %s%s
  Path:     %s
  Size:     %d bytes
  SHA256:   %s
  Recorded: %s
  Modified: %s`,
		doc.id,
		map[bool]string{true: " (retired)"}[doc.removed],
		doc.localStorage.pathRelativeToLibrary(),
		doc.contentMetadata.size,
		hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		time.Unix(int64(doc.recorded), 0).Local().Format(time.RFC1123),
		time.Unix(int64(doc.localStorage.lastModified), 0).Local().Format(time.RFC1123))
}

type jsonDoc struct {
	Dir          SemanticPath
	File         string
	Size         int64
	Sha256       string
	Recorded     unixTimestamp
	Changed      unixTimestamp
	FileModified unixTimestamp
	FileRemoved  bool
}

func (doc *document) MarshalJSON() ([]byte, error) {
	persistedDoc := jsonDoc{
		Dir:          doc.localStorage.directory,
		File:         doc.localStorage.name,
		Size:         doc.contentMetadata.size,
		Sha256:       hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		Recorded:     doc.recorded,
		Changed:      doc.changed,
		FileModified: doc.localStorage.lastModified,
		FileRemoved:  doc.removed,
	}
	return json.Marshal(persistedDoc)
}

func (docMap *DocumentIndex) UnmarshalJSON(blob []byte) error {
	var loadedMap map[DocumentId]*document
	err := json.Unmarshal(blob, &loadedMap)
	if err != nil {
		panic(err) //must not occur because persisted library's format is versioned
	}
	if *docMap == nil {
		*docMap = make(DocumentIndex, len(loadedMap))
	}
	for id, doc := range loadedMap {
		doc.id = id
		(*docMap)[id] = doc
	}
	return nil
}

func (doc *document) UnmarshalJSON(blob []byte) error {
	var loadedDoc jsonDoc
	err := json.Unmarshal(blob, &loadedDoc)
	if err != nil {
		panic(err) //must not occur because persisted library's format is versioned
	}
	doc.id = missingId
	doc.localStorage.directory = loadedDoc.Dir
	doc.localStorage.name = loadedDoc.File
	doc.localStorage.lastModified = loadedDoc.FileModified
	doc.removed = loadedDoc.FileRemoved
	doc.contentMetadata.size = loadedDoc.Size
	shaBytes, err := hex.DecodeString(loadedDoc.Sha256)
	if err != nil {
		panic(err) //must not occur unless the library has been manipulated
	}
	if len(shaBytes) != 32 {
		panic("persisted hash has bad length") //must not occur because persisted library's format is versioned
	}
	copy(doc.contentMetadata.sha256Hash[:], shaBytes)
	doc.recorded = loadedDoc.Recorded
	doc.changed = loadedDoc.Changed
	return nil
}
