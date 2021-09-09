package library

import (
	"encoding/json"

	. "github.com/n2code/doccinator/internal/document"
)

type jsonLib struct {
	LocalRoot string
	Documents map[DocumentId]*Document
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
		return err
	}
	lib.rootPath = loadedLib.LocalRoot
	for id, doc := range loadedLib.Documents {
		doc.SetId(id)
		lib.documents[id] = doc
		lib.relPathIndex[doc.Path()] = doc
	}
	return nil
}
