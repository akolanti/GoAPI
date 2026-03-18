package ragBridge

type RB_rag struct {
	Id       string
	Query    string
	Response string
	Sources  []string
	Status   string
	Err      error
	DoRetry  bool
}

type RB_trackJob struct {
	Id       string
	Query    string
	Response string
	Sources  []string
	Status   string
	Err      error
	DoRetry  bool

	TraceId string
	JobId   string
}
