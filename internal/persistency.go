package internal

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type jsonDoc struct {
	Dir          string
	File         string
	Size         int64
	Sha256       string
	Recorded     unixTimestamp
	Changed      unixTimestamp
	FileModified unixTimestamp
}

type jsonLib struct {
	LocalRoot string
	Documents map[DocumentId]*Document
}

const workInProgressFileSuffix = ".wip"
const databaseContentOpener = "LIBRARY>>>"
const databaseContentTerminator = "<<<LIBRARY"
const databaseSemanticVersion = "0.2.0"
const semVerPattern = `^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`

var semanticVersionRegex = regexp.MustCompile(semVerPattern)
var semanticVersionMajorSubmatchIndex = semanticVersionRegex.SubexpIndex("major")

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
		doc.id = id
		lib.documents[id] = doc
		lib.relPathIndex[doc.localStorage.pathRelativeToLibrary()] = doc
	}
	return nil
}

func (lib *library) SaveToLocalFile(path string, overwrite bool) {
	if !overwrite {
		if _, err := os.Lstat(path); err == nil {
			panic("library file exists, overwrite not requested")
		}
	}

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

	writeLine := func(text string) {
		_, err := compressor.Write([]byte(text + "\n"))
		if err != nil {
			panic(err)
		}
	}

	writeLine(databaseSemanticVersion)
	writeLine(databaseContentOpener)

	encoder := json.NewEncoder(compressor)
	encoder.SetIndent("", "\t")

	err = encoder.Encode(lib)
	if err != nil {
		panic(err)
	}

	writeLine(databaseContentTerminator)

	closeCompressor()
	closeFile()

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

	textUntilNewline := func() string {
		var line strings.Builder
		for {
			var char [1]byte
			_, err := decompressor.Read(char[:])
			if err != nil {
				panic(err)
			}
			if char[0] == '\n' {
				break
			}
			line.WriteByte(char[0])
		}
		return line.String()
	}

	fileVersion := textUntilNewline()
	fileVersionMatch := semanticVersionRegex.FindStringSubmatch(fileVersion)
	if fileVersionMatch == nil {
		panic("library corrupted, version not found")
	}
	appVersionMatch := semanticVersionRegex.FindStringSubmatch(databaseSemanticVersion)
	if fileVersionMatch[semanticVersionMajorSubmatchIndex] != appVersionMatch[semanticVersionMajorSubmatchIndex] {
		panic(errors.New(fmt.Sprint("incompatible persisted library version:", fileVersion)))
	}

	for textUntilNewline() != databaseContentOpener {
	}

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
	if !strings.HasPrefix(termination.String(), "\n"+databaseContentTerminator) { //newline courtesy of JSON beautification
		panic("unexpected library termination: " + termination.String())
	}
}
