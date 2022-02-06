package library

import (
	"encoding/json"

	"github.com/n2code/doccurator/internal/document"
)

type jsonLib struct {
	LocalRoot string
	Documents document.DocumentIndex
}

func (lib *library) MarshalJSON() ([]byte, error) {
	root := jsonLib{
		LocalRoot: lib.rootPath,
		Documents: lib.documents,
	}
	return json.Marshal(root)
}

func (lib *library) UnmarshalJSON(blob []byte) error {
	var loadedLib jsonLib
	err := json.Unmarshal(blob, &loadedLib)
	if err != nil {
		panic(err) //must not occur because persisted library's format is versioned
	}
	lib.rootPath = loadedLib.LocalRoot
	lib.documents = loadedLib.Documents
	for _, doc := range lib.documents {
		if !doc.IsObsolete() {
			lib.relPathActiveIndex[doc.Path()] = doc
		}
	}
	return nil
}

var pathStatusText = map[PathStatus]string{
	Error:     "Error",
	Untracked: "Untracked",
	Tracked:   "Tracked",
	Touched:   "Touched",
	Modified:  "Modified",
	Moved:     "Moved",
	Removed:   "Removed",
	Missing:   "Missing",
	Duplicate: "Duplicate",
	Obsolete:  "Obsolete",
}

func (status PathStatus) String() string {
	return pathStatusText[status]
}

func (libDoc LibraryDocument) String() string {
	doc := libDoc.library.documents[libDoc.id] //caller error if any is nil
	return doc.String()
}
