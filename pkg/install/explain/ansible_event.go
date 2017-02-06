package explain

import (
	"fmt"
	"io"

	"github.com/apprenda/kismatic/pkg/ansible"
)

// AnsibleEventStreamExplainer explains the incoming ansible event stream
type AnsibleEventStreamExplainer struct {
	// Out is the destination where the explanations are written
	Out io.Writer
	// Verbose is used to control the output level
	Verbose bool
	// EventExplainer for processing ansible events
	EventExplainer AnsibleEventExplainer
}

// Explain the incoming ansible event stream
func (e *AnsibleEventStreamExplainer) Explain(events <-chan ansible.Event) error {
	for event := range events {
		exp := e.EventExplainer.ExplainEvent(event, e.Verbose)
		if exp != "" {
			fmt.Fprint(e.Out, exp)
		}
	}
	return nil
}

// AnsibleEventExplainer explains a single event
type AnsibleEventExplainer interface {
	ExplainEvent(e ansible.Event, verbose bool) string
}
