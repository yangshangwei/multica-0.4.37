import { afterEach, expect, it } from "vitest";
import { StrictMode, useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { recoverFocus, useOpenedForm } from "./focus";

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

it("keeps the default form opener through StrictMode effect replay", async () => {
  function Form({ close }: { close: () => void }) {
    const ref = useOpenedForm<HTMLFormElement>();
    return <form ref={ref} tabIndex={-1} aria-label="Inline form"><button type="button" onClick={close}>Close form</button></form>;
  }
  function Journey() {
    const [open, setOpen] = useState(false);
    return <><button onClick={() => setOpen(true)}>Open form</button>{open && <Form close={() => setOpen(false)} />}</>;
  }
  render(<StrictMode><Journey /></StrictMode>);
  const user = userEvent.setup();
  const opener = screen.getByRole("button", { name: "Open form" });
  await user.click(opener);
  expect(screen.getByRole("form", { name: "Inline form" })).toHaveFocus();
  await user.click(screen.getByRole("button", { name: "Close form" }));
  await waitFor(() => expect(opener).toHaveFocus());
});
