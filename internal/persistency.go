package internal

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

type jsonDoc struct {
	Dir      string
	File     string
	Size     int64
	Sha256   string
	Recorded unixTimestamp
	Modified unixTimestamp
}

type jsonLib struct {
	Version     int
	LibraryRoot string
	Documents   map[DocumentId]*Document
}

func (doc *Document) MarshalJSON() ([]byte, error) {
	persistedDoc := jsonDoc{
		Dir:      doc.localStorage.directory,
		File:     doc.localStorage.name,
		Size:     doc.contentMetadata.size,
		Sha256:   base64.StdEncoding.EncodeToString(doc.contentMetadata.sha256Hash[:]),
		Recorded: doc.recorded,
		Modified: doc.localStorage.lastModified,
	}
	return json.Marshal(persistedDoc)
}

func (doc *Document) UnmarshalJSON(blob []byte) error {
	var loadedDoc jsonDoc
	err := json.Unmarshal(blob, &loadedDoc)
	if err != nil {
		return err
	}
	doc.localStorage.directory = loadedDoc.Dir
	doc.localStorage.name = loadedDoc.File
	doc.localStorage.lastModified = loadedDoc.Modified
	doc.contentMetadata.size = loadedDoc.Size
	shaBytes, err := base64.StdEncoding.DecodeString(loadedDoc.Sha256)
	if err != nil {
		panic(err)
	}
	if len(shaBytes) != 32 {
		panic("hash corrupt")
	}
	copy(doc.contentMetadata.sha256Hash[:], shaBytes)
	doc.recorded = loadedDoc.Recorded
	return nil
}

func (lib *library) MarshalJSON() ([]byte, error) {
	root := jsonLib{
		Version:     1,
		LibraryRoot: lib.rootPath,
		Documents:   lib.documents,
	}
	return json.Marshal(root)
}

func (lib *library) UnmarshalJSON(blob []byte) error {
	var loadedLib jsonLib
	err := json.Unmarshal(blob, &loadedLib)
	if err != nil {
		return err
	}
	if loadedLib.Version != 1 {
		return errors.New(fmt.Sprint("incompatible persisted library version:", loadedLib.Version))
	}
	lib.rootPath = loadedLib.LibraryRoot
	for id, doc := range loadedLib.Documents {
		doc.id = id
		lib.documents[id] = doc
		lib.relPathIndex[doc.localStorage.pathRelativeToLibrary()] = doc
	}
	return nil
}
