// Test-only in-memory hook on the pinned Playwright server's real Keyboard.
// No installed module file or product source is changed.
const { Keyboard } = require(process.env.TRIAGE_PLAYWRIGHT_SERVER_INPUT);
const original = Keyboard.prototype.insertText;
const syntheticReason = "Same upload failure; retain the customer's additional context";
Keyboard.prototype.insertText = async function(progress, text) {
  if (text === syntheticReason) {
    const mode = process.env.TRIAGE_FOCUS_CONTROL_MODE;
    if (mode === "redirect") {
      await this._page.mainFrame().evaluateExpression("window.__triageReleaseInitialFocus()");
    } else if (mode === "await-initial") {
      await this._page.mainFrame().evaluateExpression("window.__triageInspectBeforeInsert()");
    }
  }
  return original.call(this, progress, text);
};
