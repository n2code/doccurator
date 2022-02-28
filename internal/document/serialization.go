package document

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/n2code/doccurator/internal/output"
	"time"

	"github.com/n2code/ndocid"
)

func (id Id) String() string {
	return fmt.Sprintf("%s", ndocid.EncodeUint64(uint64(id)))
}

func (doc *document) String() string {
	formatTime := func(ts unixTimestamp) string {
		return time.Unix(int64(ts), 0).Local().Format(time.RFC1123)
	}
	retiredDateLine := ""
	if doc.obsolete {
		retiredDateLine = fmt.Sprintf("\n  Retired:  %s", formatTime(doc.changed))
	}
	return fmt.Sprintf(`Document %s
  Path:     %s
  Size:     %s
  SHA256:   %s
  Recorded: %s
  Modified: %s%s`,
		doc.id,
		doc.localStorage.pathRelativeToLibrary(),
		output.Filesize(doc.contentMetadata.size),
		hex.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		formatTime(doc.recorded),
		formatTime(doc.localStorage.lastModified),
		retiredDateLine)
}

type jsonDoc struct {
	Dir          SemanticPath
	File         string
	Size         int64
	Sha256       string
	Recorded     unixTimestamp
	Changed      unixTimestamp
	FileModified unixTimestamp
	FileObsolete bool
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
		FileObsolete: doc.obsolete,
	}
	return json.Marshal(persistedDoc)
}

func (docMap *Index) UnmarshalJSON(blob []byte) error {
	var loadedMap map[Id]*document
	err := json.Unmarshal(blob, &loadedMap)
	if err != nil {
		panic(err) //must not occur because persisted library's format is versioned
	}
	if *docMap == nil {
		*docMap = make(Index, len(loadedMap))
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
	doc.id = MissingId
	doc.localStorage.directory = loadedDoc.Dir
	doc.localStorage.name = loadedDoc.File
	doc.localStorage.lastModified = loadedDoc.FileModified
	doc.obsolete = loadedDoc.FileObsolete
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
