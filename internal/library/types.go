package library

import (
	"github.com/n2code/doccurator/internal/document"
)

type ignoredLibraryPath struct {
	anchored  string //relative to library root, system-native path
	directory bool   //match directories iff set else match only files
}

type library struct {
	documents               map[document.Id]document.Api
	activeAnchoredPathIndex map[string]document.Api     //active paths represent non-obsolete documents
	rootPath                string                      //absolute, system-native path
	ignoredPaths            map[ignoredLibraryPath]bool //true for all keys
}

type orderedDocuments []document.Api
type docsByRecordedAndId orderedDocuments
