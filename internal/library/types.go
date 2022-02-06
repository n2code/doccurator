package library

import (
	. "github.com/n2code/doccurator/internal/document"
)

type library struct {
	documents          map[DocumentId]DocumentApi
	relPathActiveIndex map[string]DocumentApi //active paths represent non-obsolete documents
	rootPath           string                 //absolute, system-native path
}

type orderedDocuments []DocumentApi
type docsByRecordedAndId orderedDocuments
