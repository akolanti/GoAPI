package ragBridge

type RB_rag struct {
	Id       string   `json:"id"`
	Query    string   `json:"query"`
	Response string   `json:"response"`
	Sources  []string `json:"sources"`
	Status   string   `json:"status"`
	Err      error    `json:"err,omitempty"`
	DoRetry  bool     `json:"do_retry"`
}

type RB_trackJob struct {
	Id       string   `json:"id"`
	Query    string   `json:"query"`
	Response string   `json:"response"`
	Sources  []string `json:"sources"`
	Status   string   `json:"status"`
	Err      error    `json:"err,omitempty"`
	DoRetry  bool     `json:"do_retry"`

	TraceId string `json:"trace_id"`
	JobId   string `json:"job_id"`
}
