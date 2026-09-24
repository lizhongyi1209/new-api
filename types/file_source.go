package types

import relaytypes "github.com/QuantumNous/new-api/relaykit/types"

// These aliases preserve the old package API and allow relaykit converters to
// use file sources created by legacy request DTOs without copying cached data.
type FileSource = relaytypes.FileSource
type URLSource = relaytypes.URLSource
type Base64Source = relaytypes.Base64Source
type CachedFileData = relaytypes.CachedFileData

func NewURLFileSource(url string) *URLSource {
	return relaytypes.NewURLFileSource(url)
}

func NewBase64FileSource(base64Data, mimeType string) *Base64Source {
	return relaytypes.NewBase64FileSource(base64Data, mimeType)
}

func NewFileSourceFromData(data, mimeType string) FileSource {
	return relaytypes.NewFileSourceFromData(data, mimeType)
}

func NewMemoryCachedData(base64Data, mimeType string, size int64) *CachedFileData {
	return relaytypes.NewMemoryCachedData(base64Data, mimeType, size)
}

func NewDiskCachedData(diskPath, mimeType string, size int64) *CachedFileData {
	return relaytypes.NewDiskCachedData(diskPath, mimeType, size)
}
