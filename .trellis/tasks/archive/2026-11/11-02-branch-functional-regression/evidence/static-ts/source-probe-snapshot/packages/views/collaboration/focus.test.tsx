import { afterEach, expect, it } from "vitest";
import { waitFor } from "@testing-library/react";
import { recoverFocus } from "./focus";

afterEach(() => { document.body.innerHTML = ""; });

function button(label: string) {
  const element = document.createElement("button");
  element.textContent = label;
  document.body.append(element);
  return element;
}

it("moves focus to the first live target after the focused control is removed", async () => {
  const removed = button("Delete");
  const missing = button("Gone");
  const target = button("Edit");
  missing.remove();
  removed.focus();
  removed.remove();
  recoverFocus(() => missing, () => target);
  await waitFor(() => expect(document.activeElement).toBe(target));
});

it("treats focus inside a hidden container as lost", async () => {
  const form = document.createElement("div");
  const submit = button("Send");
  form.append(submit);
  document.body.append(form);
  const panel = button("Panel");
  submit.focus();
  form.hidden = true;
  recoverFocus(() => panel);
  await waitFor(() => expect(document.activeElement).toBe(panel));
});

it("leaves focus that is still on a live control alone", async () => {
  const current = button("Current");
  const target = button("Target");
  current.focus();
  recoverFocus(() => target);
  await new Promise((resolve) => setTimeout(resolve, 50));
  expect(document.activeElement).toBe(current);
});
