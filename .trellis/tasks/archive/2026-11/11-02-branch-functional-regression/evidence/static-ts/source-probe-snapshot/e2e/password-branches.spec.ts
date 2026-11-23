import { randomUUID } from "node:crypto";
import { expect, test, type Page, type Response } from "@playwright/test";
import { TestApiClient } from "./fixtures";

const PASSWORD = "branch coverage password 2026";
const DISPLAY_NAME = "Password branch E2E";

function accountName() {
  return `pw${randomUUID().replaceAll("-", "").slice(0, 20)}`;
}

async function openRegistration(page: Page, username: string, password = PASSWORD, name = DISPLAY_NAME) {
  await page.goto("/login");
  await page.getByRole("button", { name: "Create account", exact: true }).click();
  await page.locator("#password-username").fill(username);
  await page.locator("#password-name").fill(name);
  await page.locator("#password-value").fill(password);
  await page.locator("#password-confirm").fill(password);
}

async function openLogin(page: Page, username: string, password = PASSWORD) {
  await page.goto("/login");
  await page.locator("#password-username").fill(username);
  await page.locator("#password-value").fill(password);
}

async function submit(page: Page, endpoint: "register" | "login") {
  const response = page.waitForResponse((candidate) =>
    new URL(candidate.url()).pathname === `/auth/${endpoint}` && candidate.request().method() === "POST",
  );
  await page.locator('button[type="submit"]').click();
  return response;
}

// Exhaustive value/error matrices belong in password-error.test.ts and
// server/internal/auth/password_test.go. These cases exercise browser recovery,
// actual auth responses, persisted identity and requests sent during failures.
test.describe("password authentication branch recovery", () => {
  test.skip(process.env.E2E_PASSWORD_AUTH !== "1", "Requires isolated password-mode deployment with signup enabled");

  test.beforeEach(async ({ page }) => {
    const apiBase = process.env.NEXT_PUBLIC_API_URL!;
    expect(["localhost", "127.0.0.1"]).toContain(new URL(apiBase).hostname);
    const response = await page.request.get(`${apiBase}/api/config`);
    expect(response.ok()).toBe(true);
    expect(await response.json()).toMatchObject({ auth_mode: "password", password_signup_available: true });
  });

  test("invalid username preserves registration fields and can be corrected without restarting", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    try {
      await openRegistration(page, `${username}-`);
      const rejected = await submit(page, "register");
      expect(rejected.status()).toBe(400);
      expect(await rejected.json()).toMatchObject({ code: "invalid_username" });
      await expect(page.locator("#password-error")).toHaveText(/Username can contain only English letters/);
      await expect(page.locator("#password-name")).toHaveValue(DISPLAY_NAME);
      await expect(page.locator("#password-value")).toHaveValue(PASSWORD);
      await expect(page.locator("#password-confirm")).toHaveValue(PASSWORD);
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();

      await page.locator("#password-username").fill(username);
      const registered = await submit(page, "register");
      expect(registered.status()).toBe(201);
      const result = await registered.json();
      expect((await api.loginPassword(username, PASSWORD)).id).toBe(result.user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
      await api.deletePasswordAccount(`${username}-`);
    }
  });

  test("five-character password error can be corrected to the supported six-character minimum", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    try {
      await openRegistration(page, username, "short");
      const rejected = await submit(page, "register");
      expect(rejected.status()).toBe(400);
      expect(await rejected.json()).toMatchObject({ code: "invalid_password" });
      await expect(page.locator("#password-error")).toHaveText("Password must contain at least 6 characters; you entered 5.");
      await expect(page.locator("#password-confirm")).toHaveValue("short");
      await expect(page.locator("#password-username")).toHaveValue(username);

      await page.locator("#password-value").fill("short6");
      await page.locator("#password-confirm").fill("short6");
      const registered = await submit(page, "register");
      expect(registered.status()).toBe(201);
      expect((await api.loginPassword(username, "short6")).id).toBe((await registered.json()).user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
    }
  });

  test("overlong pasted password is rejected and accepts the corrected 128-character boundary", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    const overlong = "x".repeat(129);
    const corrected = "x".repeat(128);
    try {
      await openRegistration(page, username, overlong);
      const rejected = await submit(page, "register");
      expect(rejected.status()).toBe(400);
      expect(await rejected.json()).toMatchObject({ code: "invalid_password" });
      await expect(page.locator("#password-error")).toHaveText("Password must contain no more than 128 characters; you entered 129.");
      await expect(page.locator("#password-value")).toHaveValue(overlong);
      await expect(page.locator("#password-confirm")).toHaveValue(overlong);

      await page.locator("#password-value").fill(corrected);
      await page.locator("#password-confirm").fill(corrected);
      const registered = await submit(page, "register");
      expect(registered.status()).toBe(201);
      expect((await api.loginPassword(username, corrected)).id).toBe((await registered.json()).user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
    }
  });

  test("whitespace-only display name reaches server validation and keeps credentials for correction", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    try {
      await openRegistration(page, username, PASSWORD, "   ");
      const rejected = await submit(page, "register");
      expect(rejected.status()).toBe(400);
      expect(await rejected.json()).toMatchObject({ code: "invalid_name" });
      await expect(page.locator("#password-error")).toHaveText("Enter your name. It will appear in member lists, tasks and comments.");
      await expect(page.locator("#password-value")).toHaveValue(PASSWORD);
      await expect(page.locator("#password-confirm")).toHaveValue(PASSWORD);

      await page.locator("#password-name").fill(DISPLAY_NAME);
      const registered = await submit(page, "register");
      expect(registered.status()).toBe(201);
      expect((await api.loginPassword(username, PASSWORD)).id).toBe((await registered.json()).user.id);
      expect(await api.requestJSON("/api/me")).toMatchObject({ name: DISPLAY_NAME });
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
    }
  });

  test("duplicate username after normalization leaves the original identity and password intact", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    try {
      const original = await api.registerPassword(username, PASSWORD, DISPLAY_NAME);
      await openRegistration(page, ` ${username.toUpperCase()} `, "replacement password 2026", "Replacement name");
      const rejected = await submit(page, "register");
      expect(rejected.status()).toBe(409);
      expect(await rejected.json()).toMatchObject({ code: "username_taken" });
      await expect(page.locator("#password-error")).toHaveText("This username is already taken. Choose another.");
      await expect(page.locator("#password-name")).toHaveValue("Replacement name");
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();

      await page.getByRole("button", { name: "Already have an account? Sign in", exact: true }).click();
      await expect(page.locator("#password-error")).toHaveCount(0);
      await expect(page.locator("#password-value")).toHaveValue("");
      await page.locator("#password-value").fill(PASSWORD);
      const loggedIn = await submit(page, "login");
      expect(loggedIn.status()).toBe(200);
      expect((await loggedIn.json()).user).toMatchObject({ id: original.id, name: DISPLAY_NAME, username });
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
    }
  });

  test("incorrect password preserves the login form and a corrected retry restores the same identity", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    try {
      const user = await api.registerPassword(username, PASSWORD, DISPLAY_NAME);
      await openLogin(page, username, "incorrect password 2026");
      const rejected = await submit(page, "login");
      expect(rejected.status()).toBe(401);
      expect(await rejected.json()).toMatchObject({ code: "invalid_credentials" });
      await expect(page.locator("#password-error")).toHaveText("Incorrect username or password.");
      await expect(page.locator("#password-username")).toHaveValue(username);
      await expect(page.locator("#password-value")).toHaveValue("incorrect password 2026");
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();

      await page.locator("#password-value").fill(PASSWORD);
      const loggedIn = await submit(page, "login");
      expect(loggedIn.status()).toBe(200);
      expect((await loggedIn.json()).user.id).toBe(user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
    }
  });

  test("unknown account gives a generic error and can sign in after the account is created", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    try {
      await openLogin(page, username);
      const rejected = await submit(page, "login");
      expect(rejected.status()).toBe(401);
      expect(await rejected.json()).toMatchObject({ code: "invalid_credentials" });
      await expect(page.locator("#password-error")).toHaveText("Incorrect username or password.");
      await expect(page.locator("#password-value")).toHaveValue(PASSWORD);
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();

      const user = await api.registerPassword(username, PASSWORD, DISPLAY_NAME);
      const loggedIn = await submit(page, "login");
      expect(loggedIn.status()).toBe(200);
      expect((await loggedIn.json()).user.id).toBe(user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await api.deletePasswordAccount(username);
    }
  });

  test("lost registration response offers sign-in recovery for the account the server already created", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    let registrations = 0;
    let createdUserId: string | undefined;
    try {
      await page.route("**/auth/register", async (route) => {
        if (route.request().method() !== "POST") return route.continue();
        registrations += 1;
        const response = await route.fetch();
        expect(response.status()).toBe(201);
        createdUserId = (await response.json()).user.id;
        await route.abort("failed");
      });
      await openRegistration(page, username);
      await page.locator('button[type="submit"]').click();
      await expect(page.locator("#password-error")).toHaveText("Registration could not be confirmed. Retry or sign in with the account you just created.");
      await expect(page.locator("#password-value")).toHaveValue(PASSWORD);
      await expect(page.locator("#password-confirm")).toHaveValue(PASSWORD);
      expect(createdUserId).toBeTruthy();
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();
      await page.unroute("**/auth/register");

      await page.getByRole("button", { name: "Already have an account? Sign in", exact: true }).click();
      await page.locator("#password-value").fill(PASSWORD);
      const loggedIn = await submit(page, "login");
      expect(loggedIn.status()).toBe(200);
      expect((await loggedIn.json()).user.id).toBe(createdUserId);
      expect(registrations).toBe(1);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await page.unroute("**/auth/register");
      await api.deletePasswordAccount(username);
    }
  });

  test("network failure during login keeps credentials and succeeds after connection recovery", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    let blockedRequests = 0;
    try {
      const user = await api.registerPassword(username, PASSWORD, DISPLAY_NAME);
      await openLogin(page, username);
      await page.route("**/auth/login", async (route) => {
        if (route.request().method() !== "POST") return route.continue();
        blockedRequests += 1;
        await route.abort("failed");
      });
      await page.locator('button[type="submit"]').click();
      await expect(page.locator("#password-error")).toHaveText("Cannot connect to the server. Check your network and server address, then try again.");
      await expect(page.locator("#password-username")).toHaveValue(username);
      await expect(page.locator("#password-value")).toHaveValue(PASSWORD);
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();
      expect(blockedRequests).toBe(1);
      await page.unroute("**/auth/login");

      const loggedIn = await submit(page, "login");
      expect(loggedIn.status()).toBe(200);
      expect((await loggedIn.json()).user.id).toBe(user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await page.unroute("**/auth/login");
      await api.deletePasswordAccount(username);
    }
  });

  test("temporary authentication service failure preserves the form and permits a real retry", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    let failures = 0;
    try {
      const user = await api.registerPassword(username, PASSWORD, DISPLAY_NAME);
      await openLogin(page, username);
      await page.route("**/auth/login", async (route) => {
        if (route.request().method() !== "POST") return route.continue();
        failures += 1;
        await route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "auth_unavailable", error: "Authentication service unavailable" }) });
      });
      expect((await submit(page, "login")).status()).toBe(503);
      await expect(page.locator("#password-error")).toHaveText("The account service is temporarily unavailable. Please try again later.");
      await expect(page.locator("#password-value")).toHaveValue(PASSWORD);
      await expect(page.locator('button[type="submit"]')).toBeEnabled();
      expect(failures).toBe(1);
      expect(await page.evaluate(() => localStorage.getItem("multica_token"))).toBeNull();
      await page.unroute("**/auth/login");

      const loggedIn = await submit(page, "login");
      expect(loggedIn.status()).toBe(200);
      expect((await loggedIn.json()).user.id).toBe(user.id);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await page.unroute("**/auth/login");
      await api.deletePasswordAccount(username);
    }
  });

  test("Retry-After disables login submissions until cooldown ends without discarding credentials", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    let attempts = 0;
    page.on("request", (request) => {
      if (new URL(request.url()).pathname === "/auth/login" && request.method() === "POST") attempts += 1;
    });
    try {
      const user = await api.registerPassword(username, PASSWORD, DISPLAY_NAME);
      await openLogin(page, username);
      // Preserve the deployment's real CORS policy while injecting only the
      // rate-limit result, so the browser must be able to read Retry-After.
      const config = await page.request.get(`${process.env.NEXT_PUBLIC_API_URL}/api/config`, {
        headers: { Origin: new URL(page.url()).origin },
      });
      expect(config.ok()).toBe(true);
      const corsHeaders = Object.fromEntries(Object.entries(config.headers()).filter(([key]) => key.startsWith("access-control-")));
      await page.route("**/auth/login", async (route) => {
        if (route.request().method() !== "POST") return route.continue();
        await route.fulfill({ status: 429, contentType: "application/json", headers: { ...corsHeaders, "Retry-After": "3" }, body: JSON.stringify({ code: "rate_limited", error: "Too many attempts. Try again later" }) });
      });
      const limited = await submit(page, "login");
      expect(limited.status()).toBe(429);
      expect(new URL(limited.url()).origin).not.toBe(new URL(page.url()).origin);
      await expect(page.locator("#password-error")).toHaveText("Too many attempts. Please try again later.");
      await expect(page.locator('button[type="submit"]')).toBeDisabled();
      await expect(page.locator('button[type="submit"]')).toHaveText(/Try again in [1-3] seconds/);
      await page.locator("#password-value").press("Enter");
      await expect(page.locator("#password-value")).toHaveValue(PASSWORD);
      expect(attempts).toBe(1);
      await page.unroute("**/auth/login");
      await expect(page.locator('button[type="submit"]')).toBeEnabled({ timeout: 6_000 });

      const loggedIn = await submit(page, "login");
      expect(loggedIn.status()).toBe(200);
      expect((await loggedIn.json()).user.id).toBe(user.id);
      expect(attempts).toBe(2);
      await expect(page).toHaveURL(/\/onboarding/);
    } finally {
      await page.unroute("**/auth/login");
      await api.deletePasswordAccount(username);
    }
  });

  test("pending registration locks the form and ignores repeated native submissions", async ({ page }) => {
    const username = accountName();
    const api = new TestApiClient();
    let registrations = 0;
    let release = () => {};
    let registrationResponse: Promise<Response> | undefined;
    const pending = new Promise<void>((resolve) => { release = resolve; });
    try {
      await page.route("**/auth/register", async (route) => {
        if (route.request().method() !== "POST") return route.continue();
        registrations += 1;
        await pending;
        await route.continue();
      });
      await openRegistration(page, username);
      registrationResponse = page.waitForResponse((candidate) => new URL(candidate.url()).pathname === "/auth/register" && candidate.request().method() === "POST");
      await page.locator('button[type="submit"]').click();
      await expect.poll(() => registrations).toBe(1);
      await expect(page.locator("#password-form")).toHaveAttribute("aria-busy", "true");
      for (const id of ["password-username", "password-name", "password-value", "password-confirm"]) {
        await expect(page.locator(`#${id}`)).toBeDisabled();
      }
      await expect(page.locator('button[type="submit"]')).toBeDisabled();
      await expect(page.getByRole("button", { name: "Already have an account? Sign in", exact: true })).toBeDisabled();
      // Native submissions also exercise the handler guard behind disabled UI.
      await page.locator("#password-form").evaluate((form: HTMLFormElement) => {
        form.requestSubmit();
        form.requestSubmit();
      });
      release();
      const registered = await registrationResponse;
      expect(registered.status()).toBe(201);
      expect((await api.loginPassword(username, PASSWORD)).id).toBe((await registered.json()).user.id);
      await expect(page).toHaveURL(/\/onboarding/);
      expect(registrations).toBe(1);
    } finally {
      release();
      // Finish any dispatched write before deleting the synthetic account.
      await registrationResponse?.catch(() => undefined);
      await page.unroute("**/auth/register");
      await api.deletePasswordAccount(username);
    }
  });
});
