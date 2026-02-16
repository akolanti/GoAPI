package ingest

import (
	"errors"
	"fmt"
	"time"

	"github.com/dslipak/pdf"
	"github.com/lu4p/cat"
)

func extractPDF(path string) ([]rawPage, error) {
	logger.Debug("extractPDF", "attempting extraction", path)
	f, err := pdf.Open(path)
	if err != nil {

		logger.Error("failed opening of pdf file")
		return nil, fmt.Errorf("failed to open pdf: %w", err)
	}

	var pages []rawPage
	numPages := f.NumPage()
	logger.Debug("extractPDF", "number of pages", numPages)
	for i := 1; i <= numPages; i++ {
		page := f.Page(i)
		logger.Debug("extractPDF", "page #", i)
		if page.V.IsNull() {
			logger.Debug("extractPDF", "page value is null!!")
			continue
		}

		content, err := protectExtract(page)
		logger.Debug("extractPDF", "page content", content)
		if err != nil {
			// Log warning but continue with other pages

			logger.Error("Error parsing page content", "Error", err)
			continue
		}

		pages = append(pages, rawPage{
			Number:  i,
			Content: content,
		})
	}
	return pages, nil
}

// File reads a .odt, .docx, .rtf or plaintext file and returns the content as a string
func extractdocxTxtRtf(path string) ([]rawPage, error) {

	text, err := cat.File(path)
	if err != nil {

		logger.Error("Error extracting content from doc")
		return nil, fmt.Errorf("failed to extract docx: %w", err)
	}

	//this is a bit ugly with putting all content in 1 page
	//TODO :but I will need to make my own word writer to track the pages
	return []rawPage{
		{
			Number:  1,
			Content: text,
		},
	}, nil
}

func protectExtract(page pdf.Page) (string, error) {
	type result struct {
		content string
		err     error
	}
	resChan := make(chan result, 1)

	go func() {
		content, err := page.GetPlainText(nil)
		resChan <- result{content, err}
	}()
	select {
	case r := <-resChan:
		return r.content, r.err
	case <-time.After(time.Second * 10):
		logger.Error("pageExtract", "timeout")
		return "", errors.New("timeout")
	}
}
