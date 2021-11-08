package opa

type Input struct {
	ParsedQuery *ParseQuery `json:"parsed_query"`
	ParsedPath  []string    `json:"parsed_path"`
	Attributes  *Attributes `json:"attributes"`
}
type ParseQuery struct {
	ProjectName []string `json:"projectName"`
}

type Attributes struct {
	Request *Request `json:"request"`
}

type Request struct {
	HTTP *HTTPSpec `json:"http"`
}

type HTTPSpec struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}
