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

// The UpdatingExplainer updates the terminal in real time with the ansible events received
func UpdatingExplainer(out io.Writer) *updatingExplainer {
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

func (e *updatingExplainer) ExplainEvent(ansibleEvent ansible.Event, verbose bool) string {
	switch event := ansibleEvent.(type) {
	case *ansible.PlaybookStartEvent:
		e.playCount = event.Count
		e.currentPlay = 1

	case *ansible.PlayStartEvent:
		if e.currentPlayName != "" {
			util.PrettyPrintOk(e.out.Bypass(), "%s %s", e.playCountIndicator(), e.currentPlayName)
			e.currentPlay += 1
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
	return ""
}

// DefaultEventExplainer returns the default string explanation of a given event
type DefaultEventExplainer struct {
	// Keeping this state is necessary for supporting the current way of
	// printing output to the console... I am not a fan of this, but it'll
	// do for now...
	printPlayMessage bool
	printPlayStatus  bool
	lastPlay         string
	currentTask      string
	playCount        int
	currentPlayCount int
}

func (explainer *DefaultEventExplainer) getCount() string {
	return rightPadToLen(fmt.Sprintf("%d/%d", explainer.currentPlayCount, explainer.playCount), ".", 7)
}

func rightPadToLen(s string, padStr string, overallLen int) string {
	var padCountInt int
	padCountInt = 1 + ((overallLen - len(padStr)) / len(padStr))
	var retStr = s + strings.Repeat(padStr, padCountInt)
	return retStr[:overallLen]
}

func (explainer *DefaultEventExplainer) writePlayStatus(buf io.Writer) {
	// Do not print status on the first start event or when there is an ERROR
	if explainer.printPlayStatus {
		// In regular mode print the status
		util.PrintOkln(buf)
	}
}
func (explainer *DefaultEventExplainer) writePlayStatusVerbose(buf io.Writer) {
	// In verbose mode the status is printed as a whole line after all the tasks
	// Do not print message before first play
	if explainer.printPlayMessage {
		// No tasks were printed, no nodes match the selector
		// This is OK and a valid scenario
		if explainer.printPlayStatus {
			fmt.Fprintln(buf)
			util.PrintColor(buf, util.Green, "%s Finished With No Tasks\n", explainer.lastPlay)
		} else {
			util.PrintColor(buf, util.Green, "%s  %s Finished\n", explainer.getCount(), explainer.lastPlay)
		}
		explainer.currentPlayCount = explainer.currentPlayCount + 1
	}
}

// ExplainEvent returns an explanation for the given event
func (explainer *DefaultEventExplainer) ExplainEvent(e ansible.Event, verbose bool) string {
	buf := &bytes.Buffer{}
	switch event := e.(type) {
	case *ansible.PlayStartEvent:
		// On a play start the previous play ends
		// Print a success status, but only when there were no errors
		if verbose {
			explainer.writePlayStatusVerbose(buf)
			fmt.Fprintf(buf, "%s  %s", explainer.getCount(), event.Name)
		} else {
			explainer.writePlayStatus(buf)
			// Print the play name
			util.PrettyPrint(buf, "%s  %s", explainer.getCount(), event.Name)
			explainer.currentPlayCount = explainer.currentPlayCount + 1
		}
		// Set default state for the play
		explainer.lastPlay = event.Name
		explainer.printPlayStatus = true
		explainer.printPlayMessage = true
	case *ansible.RunnerFailedEvent:
		// Print newline before first task status
		if explainer.printPlayStatus {
			fmt.Fprintln(buf)
			// Dont print play success status on error
			explainer.printPlayStatus = false
		}
		// Tasks only print at verbose level, on ERROR also print task name
		if !verbose {
			fmt.Fprintf(buf, "- Running task: %s\n", explainer.currentTask)
		}
		if event.IgnoreErrors {
			util.PrettyPrintErrorIgnored(buf, "  %s", event.Host)
		} else {
			util.PrettyPrintErr(buf, "  %s %s", event.Host, event.Result.Message)
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
	case *ansible.RunnerUnreachableEvent:
		// Host is unreachable
		// Print newline before first task
		if explainer.printPlayStatus {
			fmt.Fprintln(buf)
			// Dont print play success status on error
			explainer.printPlayStatus = false
		}
		util.PrettyPrintUnreachable(buf, "  %s", event.Host)
	case *ansible.TaskStartEvent:
		if verbose {
			// Print newline before first task status
			if explainer.printPlayStatus {
				fmt.Fprintln(buf)
				// Dont print play success status on error
				explainer.printPlayStatus = false
			}
			fmt.Fprintf(buf, "- Running task: %s\n", event.Name)
		}
		// Set current task name
		explainer.currentTask = event.Name
	case *ansible.HandlerTaskStartEvent:
		if verbose {
			// Print newline before first task
			if explainer.printPlayStatus {
				fmt.Fprintln(buf)
				// Dont print play success status on error
				explainer.printPlayStatus = false
			}
			fmt.Fprintf(buf, "- Running task: %s\n", event.Name)
		}
		// Set current task name
		explainer.currentTask = event.Name
	case *ansible.PlaybookEndEvent:
		// Playbook ends, print the last play status
		if verbose {
			explainer.writePlayStatusVerbose(buf)
		} else {
			explainer.writePlayStatus(buf)
		}
	case *ansible.RunnerSkippedEvent:
		if verbose {
			util.PrettyPrintSkipped(buf, "  %s", event.Host)
		}
	case *ansible.RunnerOKEvent:
		if verbose {
			util.PrettyPrintOk(buf, "  %s", event.Host)
		}
	case *ansible.RunnerItemOKEvent:
		msg := fmt.Sprintf("  %s", event.Host)
		if event.Result.Item != "" {
			msg = msg + fmt.Sprintf(" with %q", event.Result.Item)
		}
		if verbose {
			util.PrettyPrintOk(buf, msg)
		}
	case *ansible.RunnerItemFailedEvent:
		msg := fmt.Sprintf("  %s", event.Host)
		if event.Result.Item != "" {
			msg = msg + fmt.Sprintf(" with %q", event.Result.Item)
		}
		// Print newline before first task status
		if explainer.printPlayStatus {
			fmt.Fprintln(buf)
			// Dont print play success status on error
			explainer.printPlayStatus = false
		}
		// Tasks only print at verbose level, on ERROR also print task name
		if !verbose {
			fmt.Fprintf(buf, "- Task: %s\n", explainer.currentTask)
		}
		if event.IgnoreErrors {
			util.PrettyPrintErrorIgnored(buf, msg)
		} else {
			util.PrettyPrintErr(buf, "  %s %s", msg, event.Result.Message)
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

	case *ansible.RunnerItemRetryEvent:
		return ""
	case *ansible.PlaybookStartEvent:
		explainer.playCount = event.Count
		explainer.currentPlayCount = 1
		return ""
	default:
		if verbose {
			util.PrintColor(buf, util.Orange, "Unhandled event: %T\n", event)
		}
	}
	return buf.String()
}
