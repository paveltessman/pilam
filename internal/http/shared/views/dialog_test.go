package views

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/paveltessman/pilam/internal/platform/labels"
)

// renderedDialog writes one dialog into a string, with body as its children.
func renderedDialog(t *testing.T, props DialogProps, body templ.Component) string {
	t.Helper()

	var out strings.Builder
	ctx := templ.WithChildren(context.Background(), body)
	if err := Dialog(props, DialogCancel()).Render(ctx, &out); err != nil {
		t.Fatalf("rendering the dialog failed: %v", err)
	}
	return out.String()
}

func TestDialogCarriesTheShellAScreenEditsIn(t *testing.T) {
	props := DialogProps{Title: labels.ModelsCardTitle, Hint: labels.ModelsArticleHint}
	field := `<input name="article">`

	html := renderedDialog(t, props, templ.Raw(field))

	for _, want := range []string{
		"<dialog",
		props.Title,
		props.Hint,
		field,
		labels.ActionCancel,
		`aria-label="` + labels.ActionClose + `"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the dialog does not carry %q", want)
		}
	}
}

// The two controls that close a dialog carry the mark static/js/app.js reads.
// Nothing else closes one, so a mark that goes missing strands the user.
func TestDialogMarksBothControlsThatCloseIt(t *testing.T) {
	html := renderedDialog(t, DialogProps{Title: labels.ModelsCardTitle}, templ.NopComponent)

	if got := strings.Count(html, "data-close-dialog"); got != 2 {
		t.Errorf("controls that close the dialog = %d, want 2", got)
	}
}

func TestDialogLeavesOutTheHintLineWhenItHasNoHint(t *testing.T) {
	html := renderedDialog(t, DialogProps{Title: labels.ModelsCardTitle}, templ.NopComponent)

	if strings.Contains(html, "text-muted-foreground\">") {
		t.Errorf("the dialog renders an empty hint line: %s", html)
	}
}

// The browser centres a modal dialog with a margin of its own, and the Tailwind
// reset sets every margin to zero. Without the margin back, the dialog opens in
// the top left corner of the screen.
func TestDialogCarriesTheMarginThatCentresIt(t *testing.T) {
	html := renderedDialog(t, DialogProps{Title: labels.ModelsCardTitle}, templ.NopComponent)

	if !strings.Contains(html, "m-auto") {
		t.Errorf("the dialog does not carry the margin that centres it: %s", html)
	}
}
