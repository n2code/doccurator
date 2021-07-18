package internal

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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

const workInProgressFileSuffix = ".wip"
const databaseTerminator = "<<<DOCCINATOR-LIBRARY\n"

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

func (lib *library) SaveToLocalFile(path string) {
	tempPath := path + workInProgressFileSuffix

	file, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600|os.ModeExclusive)
	if err != nil {
		panic(err)
	}
	fileClosed := false
	closeFile := func() {
		if !fileClosed {
			err := file.Close()
			if err != nil {
				panic(err)
			}
			fileClosed = true
		}
	}
	defer closeFile()

	compressor, _ := gzip.NewWriterLevel(file, gzip.BestSpeed)
	closeCompressor := func() {
		err := compressor.Close()
		if err != nil {
			panic(err)
		}
	}
	defer closeCompressor()

	encoder := json.NewEncoder(compressor)
	encoder.SetIndent("", "\t")

	err = encoder.Encode(lib)
	if err != nil {
		panic(err)
	}

	_, err = compressor.Write([]byte(databaseTerminator))
	if err != nil {
		panic(err)
	}

	closeCompressor()
	closeFile()
	err = os.Remove(path)
	if err != nil {
		panic(err)
	}
	err = os.Rename(tempPath, path)
	if err != nil {
		panic(err)
	}
}

func (lib *library) LoadFromLocalFile(path string) {
	leftoverWorkInProgressFile := path + workInProgressFileSuffix
	_, err := os.Stat(leftoverWorkInProgressFile)
	if !errors.Is(err, os.ErrNotExist) {
		panic("old " + workInProgressFileSuffix + "-file exists, manual intervention necessary")
	}

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	decompressor, err := gzip.NewReader(file)
	if err != nil {
		panic(err)
	}
	defer decompressor.Close()

	decoder := json.NewDecoder(decompressor)
	decoder.DisallowUnknownFields()

	lib.documents = make(map[DocumentId]*Document)
	lib.relPathIndex = make(map[string]*Document)

	err = decoder.Decode(&lib)
	if err != nil {
		panic(err)
	}

	var termination strings.Builder
	io.Copy(&termination, decoder.Buffered())
	io.Copy(&termination, decompressor)
	if !strings.HasPrefix(termination.String(), "\n"+databaseTerminator) { //newline courtesy of JSON beautification
		panic("unexpected library termination: " + termination.String())
	}
}
