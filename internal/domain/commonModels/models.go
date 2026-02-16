package commonModels

import "time"

type Document struct {
	Id                  string    `json:"source_doc_id"`
	Name                string    `json:"doc_name"`
	LastIngestTimestamp time.Time `json:"ingested_at"`
	ContentType         DocType   `json:"contentType"`
}

type DocChunk struct {
	Doc                Document
	ChunkId            string `json:"chunk_id"`
	Chunk              string `json:"content"`
	PageNum            int    `json:"page_num"`
	ChunkPageOrder     int    `json:"chunk_order"`
	EmbeddingDimension string `json:"embeddingModel"`
}
type DocType string

var PDF DocType = "PDF"
var DOCX DocType = "DOCX"
var TXT DocType = "TXT"
var ERR DocType = "ERROR"
