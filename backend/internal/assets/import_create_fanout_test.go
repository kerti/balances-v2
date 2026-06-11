package assets_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// Fan-out of the create-from-file import (issue #89) to the two other asset-
// table groups, property and vehicle. The bank-account suite in
// import_create_test.go covers the shared flow exhaustively; these focus on the
// per-group field mapping (enums + typed Detail cells) and the round-trip.

func countList(t *testing.T, h *handlerHarness, path string) int {
	t.Helper()
	rec := h.do(t, "GET", path, nil)
	requireStatus(t, rec, http.StatusOK)
	return len(decodeBody[[]any](t, rec))
}

// jointPropertyDetail is a valid joint-ownership property Detail sheet. Field
// order mirrors propertyDetailFields (the export side).
func jointPropertyDetail() [][]string {
	return [][]string{
		{"display_name", "Imported house"},
		{"description", "Brought in from a file"},
		{"ownership_type", "joint"},
		{"sole_owner", ""},
		{"native_currency", "IDR"},
		{"tag", ""},
		{"property_type", "house"},
		{"address", "Jl. Mawar No. 42"},
		{"acquisition_date", "2018-06-15"},
		{"acquisition_cost", "2500000000"},
		{"annual_appreciation_rate", "2.5"},
	}
}

// jointVehicleDetail is a valid joint-ownership vehicle Detail sheet, matching
// vehicleDetailFields.
func jointVehicleDetail() [][]string {
	return [][]string{
		{"display_name", "Imported car"},
		{"description", "Brought in from a file"},
		{"ownership_type", "joint"},
		{"sole_owner", ""},
		{"native_currency", "IDR"},
		{"tag", ""},
		{"vehicle_type", "car"},
		{"make", "Toyota"},
		{"model", "Avanza"},
		{"year", "2019"},
		{"plate_number", "B 1234 XYZ"},
		{"annual_depreciation_rate", "10"},
	}
}

func TestPropertyHandlers_ImportCreate(t *testing.T) {
	t.Run("preview validates without writing", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/properties/import", buildCreateXLSX(t, jointPropertyDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.WouldCreate || body.Committed || body.ToInsert != 2 {
			t.Fatalf("want clean preview with insert=2, got %+v", body)
		}
		if countList(t, h, "/properties") != 0 {
			t.Error("preview wrote a position")
		}
	})

	t.Run("commit creates the property and seeds snapshots", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/properties/import?mode=commit", buildCreateXLSX(t, jointPropertyDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.ToInsert != 2 {
			t.Fatalf("expected committed create with id + insert=2, got %+v", body)
		}
		if countList(t, h, "/properties") != 1 {
			t.Error("commit did not persist the property")
		}
		snaps := h.do(t, "GET", "/assets/"+*body.PositionID+"/snapshots", nil)
		requireStatus(t, snaps, http.StatusOK)
		if got := decodeBody[[]any](t, snaps); len(got) != 2 {
			t.Errorf("commit did not seed 2 snapshots, got %d", len(got))
		}
	})

	t.Run("bad property_type enum is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointPropertyDetail()
		detail[6] = []string{"property_type", "mansion"} // not in the enum
		rec := h.doUpload(t, "/properties/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "property_type") {
			t.Fatalf("want a property_type field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("non-numeric acquisition_cost is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointPropertyDetail()
		detail[9] = []string{"acquisition_cost", "lots"} // not a number
		rec := h.doUpload(t, "/properties/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "acquisition_cost") {
			t.Fatalf("want an acquisition_cost field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("matching tag name resolves and is assigned", func(t *testing.T) {
		h := newHarness(t)
		tagID := h.seedTag(t, "Emergency fund")
		detail := jointPropertyDetail()
		detail[5] = []string{"tag", "Emergency fund"}
		rec := h.doUpload(t, "/properties/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("tagged commit failed: %+v", body)
		}
		got := h.do(t, "GET", "/properties/"+*body.PositionID, nil)
		requireStatus(t, got, http.StatusOK)
		assetTagID := decodeBody[struct {
			Asset struct {
				TagID *uuid.UUID `json:"tag_id"`
			} `json:"asset"`
		}](t, got).Asset.TagID
		if assetTagID == nil || *assetTagID != tagID {
			t.Fatalf("want tag_id %s, got %v", tagID, assetTagID)
		}
	})

	t.Run("unknown sole_owner email blocks creation", func(t *testing.T) {
		h := newHarness(t)
		detail := jointPropertyDetail()
		detail[2] = []string{"ownership_type", "sole"}
		detail[3] = []string{"sole_owner", "stranger@example.com"}
		rec := h.doUpload(t, "/properties/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusUnprocessableEntity)
		body := decodeBody[createImportResp](t, rec)
		if !hasFieldError(body, "sole_owner") {
			t.Fatalf("want a sole_owner field error, got %+v", body.FieldErrors)
		}
		if countList(t, h, "/properties") != 0 {
			t.Error("422 commit wrote a position")
		}
	})

	t.Run("a real export round-trips into a new property", func(t *testing.T) {
		h := newHarness(t)
		src := h.createProperty(t, "Round trip source")
		seed := h.doUpload(t, "/assets/"+src.Asset.ID.String()+"/snapshots/import?mode=commit",
			buildImportXLSX(t, [][]string{assetImportHeader, {"2026-03", "2026-03-31", "2600000000", "IDR", "Mar"}}))
		requireStatus(t, seed, http.StatusOK)

		exp := h.do(t, "GET", "/properties/"+src.Asset.ID.String()+"/export", nil)
		requireStatus(t, exp, http.StatusOK)

		rec := h.doUpload(t, "/properties/import?mode=commit", exp.Body.Bytes())
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("round-trip commit failed: %+v", body)
		}
		if *body.PositionID == src.Asset.ID.String() {
			t.Fatal("round-trip should create a NEW position, not touch the source")
		}
		if countList(t, h, "/properties") != 2 || body.ToInsert != 1 {
			t.Errorf("want 2 properties + 1 seeded snapshot, got count=%d insert=%d",
				countList(t, h, "/properties"), body.ToInsert)
		}
	})
}

func TestVehicleHandlers_ImportCreate(t *testing.T) {
	t.Run("commit creates the vehicle and seeds snapshots", func(t *testing.T) {
		h := newHarness(t)
		rec := h.doUpload(t, "/vehicles/import?mode=commit", buildCreateXLSX(t, jointVehicleDetail(), twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil || body.ToInsert != 2 {
			t.Fatalf("expected committed create with id + insert=2, got %+v", body)
		}
		if countList(t, h, "/vehicles") != 1 {
			t.Error("commit did not persist the vehicle")
		}
	})

	t.Run("bad vehicle_type enum is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointVehicleDetail()
		detail[6] = []string{"vehicle_type", "spaceship"}
		rec := h.doUpload(t, "/vehicles/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "vehicle_type") {
			t.Fatalf("want a vehicle_type field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("non-integer year is a field error", func(t *testing.T) {
		h := newHarness(t)
		detail := jointVehicleDetail()
		detail[9] = []string{"year", "twenty-nineteen"}
		rec := h.doUpload(t, "/vehicles/import", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if body.WouldCreate || !hasFieldError(body, "year") {
			t.Fatalf("want a year field error, got %+v", body.FieldErrors)
		}
	})

	t.Run("matching tag name resolves and is assigned", func(t *testing.T) {
		h := newHarness(t)
		tagID := h.seedTag(t, "Garage")
		detail := jointVehicleDetail()
		detail[5] = []string{"tag", "Garage"}
		rec := h.doUpload(t, "/vehicles/import?mode=commit", buildCreateXLSX(t, detail, twoSnapshots()))
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("tagged commit failed: %+v", body)
		}
		got := h.do(t, "GET", "/vehicles/"+*body.PositionID, nil)
		requireStatus(t, got, http.StatusOK)
		assetTagID := decodeBody[struct {
			Asset struct {
				TagID *uuid.UUID `json:"tag_id"`
			} `json:"asset"`
		}](t, got).Asset.TagID
		if assetTagID == nil || *assetTagID != tagID {
			t.Fatalf("want tag_id %s, got %v", tagID, assetTagID)
		}
	})

	t.Run("a real export round-trips into a new vehicle", func(t *testing.T) {
		h := newHarness(t)
		src := h.createVehicle(t, "Round trip source")
		seed := h.doUpload(t, "/assets/"+src.Asset.ID.String()+"/snapshots/import?mode=commit",
			buildImportXLSX(t, [][]string{assetImportHeader, {"2026-03", "2026-03-31", "180000000", "IDR", "Mar"}}))
		requireStatus(t, seed, http.StatusOK)

		exp := h.do(t, "GET", "/vehicles/"+src.Asset.ID.String()+"/export", nil)
		requireStatus(t, exp, http.StatusOK)

		rec := h.doUpload(t, "/vehicles/import?mode=commit", exp.Body.Bytes())
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[createImportResp](t, rec)
		if !body.Committed || body.PositionID == nil {
			t.Fatalf("round-trip commit failed: %+v", body)
		}
		if countList(t, h, "/vehicles") != 2 || body.ToInsert != 1 {
			t.Errorf("want 2 vehicles + 1 seeded snapshot, got count=%d insert=%d",
				countList(t, h, "/vehicles"), body.ToInsert)
		}
	})
}

func hasFieldError(body createImportResp, field string) bool {
	for _, fe := range body.FieldErrors {
		if fe.Field == field {
			return true
		}
	}
	return false
}
