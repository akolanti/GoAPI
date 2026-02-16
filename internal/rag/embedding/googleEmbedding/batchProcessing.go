package googleEmbedding

import (
	"context"

	"time"

	"github.com/akolanti/GoAPI/pkg/logger_i"
	"google.golang.org/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func getContent(chunks []string) []*genai.Content {
	contentsToSend := make([]*genai.Content, 0, len(chunks))

	for _, chunk := range chunks {
		contentsToSend = append(contentsToSend, &genai.Content{
			Parts: []*genai.Part{{Text: chunk}},
		})
	}
	return contentsToSend
}

func doRetry(err error, log *logger_i.Logger) bool {
	if s, ok := status.FromError(err); ok {
		if s.Code() == codes.ResourceExhausted {
			log.Error("Rate limit hit! ", "error", err)
			return true
		}
	}
	return false
}

func getInlinedBatchRequests(chunks []string) *genai.EmbedContentBatch {
	conf := genai.EmbedContentConfig{OutputDimensionality: &dimension}
	embedContentBatch := genai.EmbedContentBatch{
		Config:   &conf,
		Contents: getContent(chunks),
	}
	return &embedContentBatch
}

func (c *client) pollForAnswer(ctx context.Context, batchJobName string, log *logger_i.Logger) (*genai.BatchJob, error) {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	log.Debug("pollForAnswer")
	for {
		select {
		case <-ctx.Done():
			log.Error("pollForAnswer cancelled", "error:", ctx.Err())
			return nil, ctx.Err()

		case <-ticker.C:

			bJob, err := c.genAi.Batches.Get(ctx, batchJobName, nil)
			if err != nil {
				logger.Error("Error getting batch job:", "error", err)
			}

			//https://pkg.go.dev/google.golang.org/genai@v1.41.1#JobState
			switch bJob.State {
			case "JOB_STATE_SUCCEEDED":
				log.Debug("batch job succeeded")
				return bJob, nil

			case "JOB_STATE_FAILED":
				log.Error("batch job failed :", "JOB_STATE_FAILED", bJob.Error.Message)
			case "JOB_STATE_CANCELLED", "JOB_STATE_EXPIRED", "JOB_STATE_PARTIALLY_SUCCEEDED":
				log.Error("batch job failed :", "Premature ending", bJob.State)
				//all other states we will wait for context to expire or job to end
			}
		}
	}

}

func downloadAnswerFromClient(answer *genai.BatchJob, logger *logger_i.Logger) ([][]float32, error) {
	res := answer.Dest.InlinedEmbedContentResponses
	if res == nil || len(res) == 0 {
		return [][]float32{}, nil
	}
	var results [][]float32

	for _, r := range res {
		var val []float32
		if r == nil || r.Error != nil || r.Response == nil || r.Response.Embedding == nil {
			//TODO:an embedding result can fail, this is needs to be handled
			//https://pkg.go.dev/google.golang.org/genai@v1.41.1#ContentEmbedding
			logger.Error("Error with a particular result in batch embedding", "error", r)
			val = nil
		} else {
			val = r.Response.Embedding.Values
		}
		results = append(results, val)
	}
	return results, nil
}
