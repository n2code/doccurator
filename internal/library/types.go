package library

import (
	"github.com/n2code/doccurator/internal/document"
)

type library struct {
	documents          map[document.DocumentId]document.DocumentApi
	relPathActiveIndex map[string]document.DocumentApi //active paths represent non-obsolete documents
	rootPath           string                          //absolute, system-native path
}

type orderedDocuments []document.DocumentApi
type docsByRecordedAndId orderedDocuments
