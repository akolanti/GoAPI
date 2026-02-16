package ingest

import (
	"context"
	"os"
	"time"

	"github.com/akolanti/GoAPI/internal/config"
	"github.com/akolanti/GoAPI/internal/domain/commonModels"
	"github.com/akolanti/GoAPI/internal/domain/jobModel"
	"github.com/akolanti/GoAPI/internal/rag/embedding"
	"github.com/akolanti/GoAPI/internal/rag/vectorDB"
	"github.com/akolanti/GoAPI/pkg/logger_i"
)

type rawPage struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

var logger *logger_i.Logger

func ProcessDocumentIngestion(ctx context.Context, job jobModel.Job, e embedding.Embedder, vectorDatabase vectorDB.DataProcessor) jobModel.Job {
	logger = logger_i.NewLogger("Document Ingestion ")
	logger.With("traceId", ctx.Value(config.TRACE_ID_KEY).(string))

	//ideally return batches of upserts
	docName := job.JobPayload.IngestFileName
	docPath := job.JobPayload.IngestURL

	logger.Debug("Processing document", "filename", docName, "path", docPath)

	job.CurrentStep = jobModel.IngestProcessing
	err := vectorDatabase.CreateCollection(ctx, config.EmbeddingDBName)
	if err != nil {
		logger.Error("Error creating collection", "error", err)
		job.Status = jobModel.JobStatusError
		return job
	}

	docType := getDocType(docPath)
	logger.Debug("Processing document", "type", docType)
	if docType == commonModels.ERR {
		logger.Error("Error getting document type", "error", err)
		job.Status = jobModel.JobStatusError
		return job
	}

	doc := commonModels.Document{
		Id:                  job.Id,
		Name:                docName,
		LastIngestTimestamp: time.Now(),
		ContentType:         docType,
	}

	rawPages, err := extractText(job.JobPayload.IngestURL, doc.ContentType)
	if err != nil {
		logger.Error("Error processing document", "error", err)
		job.Status = jobModel.JobStatusError
		job.Error.Message = "Error extracting document content"
		return job
	}

	logger.Debug("Processing document", "Number of raw pages: ", len(rawPages))
	chunks := PrepareChunks(rawPages, doc, "temp model")

	logger.Debug("Processing document", "Number of chunks: ", len(chunks))
	err = BatchIngest(ctx, chunks, vectorDatabase, e)

	if err != nil {
		job.Status = jobModel.JobStatusError
		logger.Error("Error processing document", "error", err)
		return job
	}
	err = os.Remove(job.JobPayload.IngestURL)
	if err != nil {
		logger.Error("Error removing file", "error", err)
	}
	job.Status = jobModel.JobStatusComplete
	return job
}
