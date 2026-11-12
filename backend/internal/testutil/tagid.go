package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TagIDCase is one position subtype's HTTP surface for RunTagIDContract.
type TagIDCase struct {
	Name string
	// Path is the collection route (POST creates, PATCH Path/{id} updates).
	Path string
	// Body is a valid create body. The same body serves the PATCH: the update
	// requests ignore the create-only keys (currency etc.).
	Body map[string]any
	// Core names the JSON key that holds the row carrying tag_id ("asset",
	// "investment") or "" when the row is the response body itself.
	Core string
}

// RunTagIDContract drives the wire contract of the optional tag_id field on a
// position's POST and PATCH (issue #839): a uuid assigns, null clears, absent
// leaves the Tag alone on update, a malformed uuid is a 400, and a Tag from
// another household is a 404 that writes nothing. own is a Tag of the caller's
// household, foreign one of another's.
func RunTagIDContract(t *testing.T, do func(method, path string, body any) *httptest.ResponseRecorder, own, foreign string, cases []TagIDCase) {
	t.Helper()

	with := func(base map[string]any, extra map[string]any) map[string]any {
		out := make(map[string]any, len(base)+len(extra))
		for k, v := range base {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	decode := func(t *testing.T, rec *httptest.ResponseRecorder, c TagIDCase) (id string, tag any) {
		t.Helper()
		var raw map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
		row := raw
		if c.Core != "" {
			row, _ = raw[c.Core].(map[string]any)
		}
		id, _ = row["id"].(string)
		return id, row["tag_id"]
	}
	want := func(t *testing.T, step string, got any, wantTag string) {
		t.Helper()
		switch {
		case wantTag == "" && got != nil:
			t.Errorf("%s: tag_id = %v, want null", step, got)
		case wantTag != "" && got != wantTag:
			t.Errorf("%s: tag_id = %v, want %s", step, got, wantTag)
		}
	}
	status := func(t *testing.T, step string, rec *httptest.ResponseRecorder, code int) {
		t.Helper()
		if rec.Code != code {
			t.Fatalf("%s: status %d, want %d (body: %s)", step, rec.Code, code, rec.Body.String())
		}
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			rec := do(http.MethodPost, c.Path, with(c.Body, map[string]any{"tag_id": own}))
			status(t, "create with own tag", rec, http.StatusCreated)
			id, tag := decode(t, rec, c)
			want(t, "create with own tag", tag, own)
			path := c.Path + "/" + id

			rec = do(http.MethodPost, c.Path, with(c.Body, map[string]any{"tag_id": nil}))
			status(t, "create with null tag", rec, http.StatusCreated)
			_, tag = decode(t, rec, c)
			want(t, "create with null tag", tag, "")

			rec = do(http.MethodPost, c.Path, c.Body)
			status(t, "create without tag_id", rec, http.StatusCreated)

			if rec = do(http.MethodPost, c.Path, with(c.Body, map[string]any{"tag_id": foreign})); rec.Code != http.StatusNotFound {
				t.Errorf("create with foreign tag: status %d, want 404 (body: %s)", rec.Code, rec.Body.String())
			}
			if rec = do(http.MethodPost, c.Path, with(c.Body, map[string]any{"tag_id": "not-a-uuid"})); rec.Code != http.StatusBadRequest {
				t.Errorf("create with malformed tag: status %d, want 400 (body: %s)", rec.Code, rec.Body.String())
			}

			// Absent leaves the Tag alone while the rest of the update applies.
			rec = do(http.MethodPatch, path, with(c.Body, map[string]any{"display_name": "renamed"}))
			status(t, "update without tag_id", rec, http.StatusOK)
			_, tag = decode(t, rec, c)
			want(t, "update without tag_id", tag, own)

			rec = do(http.MethodPatch, path, with(c.Body, map[string]any{"tag_id": nil}))
			status(t, "update with null tag", rec, http.StatusOK)
			_, tag = decode(t, rec, c)
			want(t, "update with null tag", tag, "")

			rec = do(http.MethodPatch, path, with(c.Body, map[string]any{"tag_id": own}))
			status(t, "update with own tag", rec, http.StatusOK)
			_, tag = decode(t, rec, c)
			want(t, "update with own tag", tag, own)

			if rec = do(http.MethodPatch, path, with(c.Body, map[string]any{"tag_id": foreign})); rec.Code != http.StatusNotFound {
				t.Errorf("update with foreign tag: status %d, want 404 (body: %s)", rec.Code, rec.Body.String())
			}
			// The failed write changed nothing: the Tag is still ours.
			rec = do(http.MethodGet, path, nil)
			status(t, "get after rejected update", rec, http.StatusOK)
			_, tag = decode(t, rec, c)
			want(t, "after rejected update", tag, own)
		})
	}
}
