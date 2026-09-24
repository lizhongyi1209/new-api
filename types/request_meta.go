package types

import relaytypes "github.com/QuantumNous/new-api/relaykit/types"

// Keep legacy request types identical to relaykit types while callers migrate.
type FileType = relaytypes.FileType

const (
	FileTypeImage = relaytypes.FileTypeImage
	FileTypeAudio = relaytypes.FileTypeAudio
	FileTypeVideo = relaytypes.FileTypeVideo
	FileTypeFile  = relaytypes.FileTypeFile
)

type TokenType = relaytypes.TokenType

const (
	TokenTypeTextNumber = relaytypes.TokenTypeTextNumber
	TokenTypeTokenizer  = relaytypes.TokenTypeTokenizer
	TokenTypeImage      = relaytypes.TokenTypeImage
)

type TokenCountMeta = relaytypes.TokenCountMeta
type FileMeta = relaytypes.FileMeta
type RequestMeta = relaytypes.RequestMeta

func NewFileMeta(fileType FileType, source FileSource) *FileMeta {
	return relaytypes.NewFileMeta(fileType, source)
}

func NewImageFileMeta(source FileSource, detail string) *FileMeta {
	return relaytypes.NewImageFileMeta(source, detail)
}
