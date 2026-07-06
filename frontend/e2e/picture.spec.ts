import { test, expect } from "@playwright/test";

// Proves the Google-picture backfill end-to-end: seeded Alice has picture_url
// NULL, so the header avatar renders initials. After signing in through mock-oidc
// — which includes a `picture` claim in its id_token (mockoidc.go) — the OAuth
// callback runs SetUserPicture and the avatar swaps to <img>. This is the one
// path session-injection can't cover (the backfill only happens during a real
// sign-in). See ADR-0024.
//
// Override the project's injected storageState with an empty one so the context
// starts unauthenticated and the SignInScreen renders first. Note: this spec
// leaves Alice's picture_url populated for the rest of the suite — no other
// spec depends on it being NULL, mirroring login.spec.ts's persisted-session
// approach.
test.use({ storageState: { cookies: [], origins: [] } });

test("OAuth callback backfills picture_url and the avatar renders the image", async ({ page }) => {
  await page.goto("/");

  const signIn = page.getByTestId("signin-google");
  await expect(signIn).toBeVisible();
  await signIn.click();

  // Authenticated shell renders; assert the seeded Alice landed.
  await expect(page.getByText("Alice", { exact: true })).toBeVisible();

  // The avatar's image branch is now active, with the URL from the id_token's
  // `picture` claim. The initials fallback must be gone.
  const img = page.getByTestId("user-avatar-img");
  await expect(img).toBeVisible();
  await expect(img).toHaveAttribute("src", "http://localhost:8090/avatar.png");
  await expect(page.getByTestId("user-avatar-fallback")).toHaveCount(0);
});
