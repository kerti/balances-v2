package testutil

import (
	"context"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// CreateHouseholdWithUser creates a household and a single user within it,
// bypassing the OAuth flow. Returns the persisted User row, suitable for
// passing into identity.WithUser to simulate an authenticated request.
func CreateHouseholdWithUser(t *testing.T, q *db.Queries, displayName string) db.User {
	t.Helper()
	ctx := context.Background()

	household, err := q.CreateHousehold(ctx, db.CreateHouseholdParams{
		DisplayName:       displayName + "'s Household",
		ReportingCurrency: "IDR",
	})
	if err != nil {
		t.Fatalf("CreateHousehold(%s): %v", displayName, err)
	}

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		HouseholdID: household.ID,
		DisplayName: displayName,
		Email:       displayName + "@example.com",
		GoogleSub:   "test-sub-" + displayName,
		Locale:      "id-ID",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   nil,
	})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", displayName, err)
	}

	return user
}
