package feature_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	permission "github.com/hyz-is/arandu-permission"
)

// TestTheSummaryOfAGroupAndOfAPersonDrawsTheTokenOfItsConfirmationForm holds
// the two fragments the panel swaps in before a bulk write: the group's summary
// and the person's. Each holds a form, the confirmation, and the markup of a
// form begins with @csrf.
//
// @csrf asks the data the view was handed for CSRFToken, and a fragment is not
// a page that embeds the chrome, so nothing gave it one: the summary answered
// 500 for an administrator who had done nothing wrong, and no test noticed
// because none of them drew it. The renderer of these tests calls the same
// function the compiled markup does, so a fragment without the token fails here
// as it fails in the application, and the token it draws is the one the
// middleware accepts on the form's submission.
func TestTheSummaryOfAGroupAndOfAPersonDrawsTheTokenOfItsConfirmationForm(t *testing.T) {
	t.Parallel()

	app := serve(t, true, nil)
	session := app.signIn(t, administrator)

	page, err := app.svc.ListGroups(context.Background(), app.admin, permission.GroupListQuery{Limit: 100})
	if err != nil || len(page.Items) == 0 {
		t.Fatalf("reading the seeded group: %v (%d found)", err, len(page.Items))
	}
	group := page.Items[0].ID

	token := app.screen(t, session)
	for _, summary := range []struct {
		name   string
		target string
		form   url.Values
	}{
		{"a group's actions", permission.DefaultPrefix + "/groups/" + group + "/summary", url.Values{"kind": {"actions"}, "value": {"invoice.delete"}}},
		{"a group's members", permission.DefaultPrefix + "/groups/" + group + "/summary", url.Values{"kind": {"members"}, "value": {"user-1", "user-9"}}},
		{"a person's own actions", permission.DefaultPrefix + "/users/user-9/summary", url.Values{"value": {"invoice.delete"}}},
	} {
		summary.form.Set("_token", token)
		rec := app.visit(t, http.MethodPost, summary.target, summary.form, session, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("the summary of %s answered %d, want %d: %s", summary.name, rec.Code, http.StatusOK, rec.Body)
			continue
		}
		if got := field(t, rec.Body.String(), "_token"); got == "" {
			t.Errorf("the summary of %s drew an empty token, so confirming it is refused", summary.name)
		}
	}
}
