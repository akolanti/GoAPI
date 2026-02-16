package ingest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/akolanti/GoAPI/internal/adapter/utils"
	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/commonModels"
	"github.com/akolanti/GoAPI/internal/rag/embedding"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

//splitter

func splitTextIntoChunks(text string, limit int, overlap int) []string {
	var chunks []string

	// If text is already small enough, just return it
	if len(text) <= limit {
		return []string{text}
	}

	// Separators ordered from "best" to "worst" for semantic meaning
	separators := []string{"\n\n", "\n", ". ", " ", ""}

	var splitChar string
	found := false
	for _, s := range separators {
		if strings.Contains(text, s) {
			splitChar = s
			found = true
			break
		}
	}

	if !found {
		// Hard cut if no separator found (rare)
		return []string{text[:limit]}
	}

	parts := strings.Split(text, splitChar)
	var currentChunk strings.Builder

	for _, part := range parts {
		// If adding the part exceeds the limit
		if currentChunk.Len()+len(part)+len(splitChar) > limit {
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
			}

			// Handle overlap: start the next chunk with the end of the previous one
			// (Simple version: take last N chars)
			overlapContent := ""
			if currentChunk.Len() > overlap {
				overlapContent = currentChunk.String()[currentChunk.Len()-overlap:]
			}

			currentChunk.Reset()
			currentChunk.WriteString(overlapContent)
		}

		if currentChunk.Len() > 0 && splitChar != "" {
			currentChunk.WriteString(splitChar)
		}
		currentChunk.WriteString(part)
	}

	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

func getDocType(docPath string) commonModels.DocType {
	ext := strings.ToLower(filepath.Ext(docPath))
	switch ext {
	case ".pdf":
		return commonModels.PDF
	case ".docx", ".txt", ".rtf":
		return commonModels.DOCX
	default:
		return commonModels.ERR
	}
}

func extractText(url string, contentType commonModels.DocType) ([]rawPage, error) {
	switch contentType {
	case commonModels.PDF:
		return extractPDF(url)
	case commonModels.DOCX:
		return extractdocxTxtRtf(url)

	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

func PrepareChunks(pages []rawPage, doc commonModels.Document, embeddingModel string) []commonModels.DocChunk {
	var allChunks []commonModels.DocChunk

	// Limits for the splitter
	const maxChunkSize = 1000 // characters
	const overlap = 150       // generous overlap helps semantic continuity

	for _, page := range pages {
		// 1. Split the text of this specific page
		stringChunks := splitTextIntoChunks(page.Content, maxChunkSize, overlap)

		// 2. Map strings into your DocChunk struct
		for i, text := range stringChunks {
			allChunks = append(allChunks, commonModels.DocChunk{
				Doc:                doc,
				ChunkId:            utils.GetNewUUID(),
				Chunk:              text,
				PageNum:            page.Number,
				ChunkPageOrder:     i,
				EmbeddingDimension: embeddingModel, //this can help us later if we want to have multiple embedding models

			})
		}
	}

	return allChunks
}

func BatchIngest(ctx context.Context, chunks []commonModels.DocChunk, vectorDB vectorDB.DataProcessor, embedder embedding.Embedder) error {
	logger = logger_i.NewLogger("Batch Ingestion ")
	logger.With("traceId", ctx.Value(config.TRACE_ID_KEY).(string))

	batchSize := 100
	isHugeDataSet := false

	if len(chunks) > 1000000 { //we only want to do this if there is a huge document
		isHugeDataSet = true
		logger.Debug("Is a huge dataset")
	}

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		currentBatch := chunks[i:end]

		//TODO:each batch can be its own go routine
		//but we will monitor the memory before introducing its own worker routine

		// 1. Extract just the text strings for the embedder
		var texts []string
		for _, c := range currentBatch {
			if c.Chunk != "" {
				texts = append(texts, c.Chunk)
			}
		}

		logger.Debug("Staring embedding call", "current batch length ", len(currentBatch), "length of texts", len(texts))
		// vectors is [][]float32
		vectors, err := embedder.BatchEmbedding(ctx, texts, isHugeDataSet)
		if err != nil {
			return fmt.Errorf("embedding batch failed: %w", err)
		}

		// 4. Upsert the batch to Qdrant
		err = vectorDB.UpsertBatch(ctx, config.EmbeddingDBName, currentBatch, vectors)
		if err != nil {
			return fmt.Errorf("upserting to qdrant failed: %w", err)
		}
	}

	return nil
}
