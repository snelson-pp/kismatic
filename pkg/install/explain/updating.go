package explain

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/apprenda/kismatic/pkg/ansible"
	"github.com/apprenda/kismatic/pkg/util"
	"github.com/gosuri/uilive"
)

// DefaultExplainer returns the default ansible explainer
func DefaultExplainer(verbose bool, out io.Writer) AnsibleEventExplainer {
	if verbose {
		return &verboseExplainer{out: out}
	}
	// otherwise, return the updating explainer
	w := uilive.New()
	w.Out = out
	w.Start()
	return &updatingExplainer{
		out: w,
	}
}

type updatingExplainer struct {
	out             *uilive.Writer
	playCount       int
	currentPlay     int
	currentPlayName string
	currentTask     string
	failureOccurred bool
}

func (e *updatingExplainer) playCountIndicator() string {
	return rightPadToLen(fmt.Sprintf("%d/%d", e.currentPlay, e.playCount), ".", 7)
}

func (e *updatingExplainer) ExplainEvent(ansibleEvent ansible.Event) {
	switch event := ansibleEvent.(type) {
	case *ansible.PlaybookStartEvent:
		e.playCount = event.Count
		e.currentPlay = 1

	case *ansible.PlayStartEvent:
		if e.currentPlayName != "" {
			util.PrettyPrintOk(e.out.Bypass(), "%s %s", e.playCountIndicator(), e.currentPlayName)
			e.currentPlay++
		}
		e.currentPlayName = event.Name
		fmt.Fprintln(e.out, e.playCountIndicator(), e.currentPlayName)

	case *ansible.PlaybookEndEvent:
		// Assuming no failure detected: playbook end => previous play success
		if !e.failureOccurred {
			util.PrettyPrintOk(e.out.Bypass(), "%s %s", e.playCountIndicator(), e.currentPlayName)
		}

	case *ansible.TaskStartEvent:
		e.currentTask = event.Name
		buf := &bytes.Buffer{}
		fmt.Fprintln(buf, e.playCountIndicator(), e.currentPlayName)
		fmt.Fprintln(buf, "- Task:", e.currentTask)
		e.out.Write(buf.Bytes())

	case *ansible.HandlerTaskStartEvent:
		// Ansible echoes events for handlers even if the previous handler
		// did not run successfully. We write handler information only if
		// no failure has occurred.
		if !e.failureOccurred {
			buf := &bytes.Buffer{}
			fmt.Fprintln(buf, e.playCountIndicator(), e.currentPlayName)
			fmt.Fprintln(buf, "- Task: ", event.Name)
			e.out.Write(buf.Bytes())
		}

	case *ansible.RunnerOKEvent:
		buf := &bytes.Buffer{}
		fmt.Fprintln(buf, e.playCountIndicator(), e.currentPlayName)
		util.PrettyPrintOk(buf, "- %s %s", event.Host, e.currentTask)
		e.out.Write(buf.Bytes())

	case *ansible.RunnerItemOKEvent:
		buf := &bytes.Buffer{}
		fmt.Fprintln(buf, e.playCountIndicator(), e.currentPlayName)
		msg := fmt.Sprintf("  %s", event.Host)
		if event.Result.Item != "" {
			msg = msg + fmt.Sprintf(" with %q", event.Result.Item)
		}
		util.PrettyPrintOk(buf, msg)
		e.out.Write(buf.Bytes())

	case *ansible.RunnerFailedEvent:
		buf := &bytes.Buffer{}
		// Only print this header if this is the first failure we get
		if !e.failureOccurred {
			util.PrettyPrintErr(buf, "%s %s", e.playCountIndicator(), e.currentPlayName)
			fmt.Fprintln(buf, "- Task: "+e.currentTask)
		}
		if event.IgnoreErrors {
			util.PrettyPrintErrorIgnored(buf, "  %s", event.Host)
		} else {
			util.PrettyPrintErr(buf, "  %s: %s", event.Host, event.Result.Message)
		}
		if event.Result.Stdout != "" {
			util.PrintColor(buf, util.Red, "---- STDOUT ----\n%s\n", event.Result.Stdout)
		}
		if event.Result.Stderr != "" {
			util.PrintColor(buf, util.Red, "---- STDERR ----\n%s\n", event.Result.Stderr)
		}
		if event.Result.Stderr != "" || event.Result.Stdout != "" {
			util.PrintColor(buf, util.Red, "---------------\n")
		}
		fmt.Fprintf(e.out.Bypass(), buf.String())
		e.failureOccurred = true
	case *ansible.RunnerUnreachableEvent:
		fmt.Fprintln(e.out.Bypass(), e.playCountIndicator(), e.currentPlayName)
		util.PrettyPrintUnreachable(e.out.Bypass(), "  %s", event.Host)

	case *ansible.RunnerSkippedEvent:
		buf := &bytes.Buffer{}
		fmt.Fprintln(buf, e.playCountIndicator(), e.currentPlayName)
		util.PrettyPrintSkipped(buf, "- %s %s", event.Host, e.currentTask)
		e.out.Write(buf.Bytes())

	case *ansible.RunnerItemFailedEvent:
		buf := &bytes.Buffer{}
		// Only print this header if this is the first failure we get
		if !e.failureOccurred {
			util.PrettyPrintErr(buf, "%s %s", e.playCountIndicator(), e.currentPlayName)
			fmt.Fprintln(buf, "- Task: "+e.currentTask)
		}
		msg := fmt.Sprintf("  %s", event.Host)
		if event.Result.Item != "" {
			msg = msg + fmt.Sprintf(" with %q", event.Result.Item)
		}
		if event.IgnoreErrors {
			util.PrettyPrintErrorIgnored(buf, msg)
		} else {
			util.PrettyPrintErr(buf, "  %s: %s", msg, event.Result.Message)
		}
		if event.Result.Stdout != "" {
			util.PrintColor(buf, util.Red, "---- STDOUT ----\n%s\n", event.Result.Stdout)
		}
		if event.Result.Stderr != "" {
			util.PrintColor(buf, util.Red, "---- STDERR ----\n%s\n", event.Result.Stderr)
		}
		if event.Result.Stderr != "" || event.Result.Stdout != "" {
			util.PrintColor(buf, util.Red, "---------------\n")
		}
		fmt.Fprintf(e.out.Bypass(), buf.String())
		e.failureOccurred = true

	case *ansible.RunnerItemRetryEvent:
		buf := &bytes.Buffer{}
		fmt.Fprintln(buf, e.playCountIndicator(), e.currentPlayName)
		fmt.Fprintf(buf, "- [%s] Retrying: %s (%d/%d attempts)\n", event.Host, e.currentTask, event.Result.Attempts, event.Result.MaxRetries)
		e.out.Write(buf.Bytes())

	default:
		util.PrintColor(e.out.Bypass(), util.Orange, "Unhandled event: %T\n", event)
	}
}

func rightPadToLen(s string, padStr string, overallLen int) string {
	var padCountInt int
	padCountInt = 1 + ((overallLen - len(padStr)) / len(padStr))
	var retStr = s + strings.Repeat(padStr, padCountInt)
	return retStr[:overallLen]
}
