package recorder

import (
	"io/ioutil"
	"net/http"

	"github.com/RedTeamPentesting/monsoon/request"
)

// Template is the template used to construct an HTTP request.
type Template struct {
	URL    string      `json:"url"`
	Method string      `json:"method"`
	Body   string      `json:"body,omitempty"`
	Header http.Header `json:"header"`
}

// NewTemplate builds a template to write to the JSON data file.
func NewTemplate(request *request.Request) (t Template, err error) {
	req, err := request.Apply(request.Replace)
	if err != nil {
		return Template{}, err
	}

	t.URL = req.URL.String()
	t.Method = req.Method
	t.Header = req.Header

	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return Template{}, err
	}
	t.Body = string(buf)

	return t, nil
}
