await locale("zh-Hans");
await theme("light");
await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
await control("/viewport", { width: 886, height: 540, zoom: 1 });
const currentPath = await navigate("current");
const listPath = currentPath.split("/autopilots")[0] + "/autopilots";
const pickerPath = listPath + "/new/template";
const results = { back: [], toolbarOwnership: [], refetch: [], touch: [] };
const save = async () => fs.writeFile(path.join(dir, "interactions.json"), JSON.stringify(results, null, 2) + "\n");
const state = async () => root().evaluate((node) => {
  const active = document.activeElement;
  const result = {
    scroll: node.scrollTop,
    group: node.querySelector('[aria-pressed="true"]')?.getAttribute("aria-label"),
    focus: { tag: active.tagName, label: active.getAttribute("aria-label"), title: active.matches("li > button") ? active.querySelector(".text-title-sm")?.textContent : null },
  };
  let fiber = node[Object.keys(node).find((key) => key.startsWith("__reactFiber"))];
  while (fiber) {
    const value = fiber.memoizedProps?.value;
    if (typeof value?.getViewState === "function") {
      result.pending = value.getViewState("autopilot-template-focus") ?? null;
      break;
    }
    fiber = fiber.return;
  }
  return result;
});
const finalRoute = async (origin, names) => {
  await expect(root()).toBeVisible();
  if (origin === "empty-list") await expect(page.getByRole("heading", { name: names.page.new_autopilot, exact: true })).toHaveCount(0);
  // The original native gap restored a temporary card for one frame. Check
  // ownership after the route has committed and retain its node identity.
  await page.waitForTimeout(200);
};
const enter = async (origin, names) => {
  await navigate("push", origin === "empty-list" ? listPath : pickerPath);
  await finalRoute(origin, names);
  await root().getByRole("button", { name: `${names.catalog.groups.maintenance} 4`, exact: true }).click();
  await root().getByRole("group").getByRole("button").last().focus();
  for (let index = 0; index < 4; index++) await page.keyboard.press("Tab");
  const before = await state();
  expect(before.focus.tag).toBe("BUTTON");
  expect(before.scroll).toBeGreaterThan(0);
  await page.keyboard.press("Enter");
  await expect(root()).toHaveCount(0);
  await expect(page.getByRole("button", { name: names.template_picker.enable, exact: true })).toBeDisabled();
  return before;
};

for (const language of ["zh-Hans", "en"]) {
  await locale(language);
  const names = JSON.parse(await fs.readFile(path.join(process.cwd(), "packages/views/locales", language, "autopilots.json"), "utf8"));
  for (const origin of ["empty-list", "creation-gallery"]) {
    for (const action of ["in-flow", "native-keyboard"]) {
      const before = await enter(origin, names);
      if (action === "in-flow") {
        const header = page.locator("header").filter({ has: page.getByRole("heading", { name: names.page.new_autopilot, exact: true }) });
        await header.getByRole("button", { name: names.template_picker.back, exact: true }).focus();
        await page.keyboard.press("Enter");
      } else {
        await page.keyboard.press("Meta+ArrowLeft");
      }
      await finalRoute(origin, names);
      const after = await state();
      expect(after.group).toBe(before.group);
      expect(after.scroll).toBe(before.scroll);
      expect(after.focus).toEqual(before.focus);
      expect(after.pending).toBe(null);
      const target = await root().locator("li > button").last().elementHandle();
      await page.waitForTimeout(100);
      expect(await target.evaluate((node) => node.isConnected && document.activeElement === node)).toBe(true);
      const screenshot = await shot(`${language === "en" ? "en" : "zh"}-${origin}-${action}-return`);
      results.back.push({ language, origin, action, before, after, sameConnectedNodeAfter100ms: true, screenshot: path.basename(screenshot) });
      await save();
      console.log(JSON.stringify({ language, origin, action, restored: true, scroll: after.scroll, focus: after.focus.title }));
    }

    await enter(origin, names);
    const nativeBack = page.getByRole("button", { name: "Go back", exact: true }).filter({ has: page.locator("svg") }).first();
    await nativeBack.click();
    await finalRoute(origin, names);
    await expect(nativeBack).toBeFocused();
    expect((await state()).pending).toBe(null);
    results.toolbarOwnership.push({ language, origin, retainedToolbarFocus: true, pendingConsumed: true });

    const deliberateOwner = root().getByRole("button", { name: names.page.start_blank, exact: true });
    await deliberateOwner.focus();
    await page.evaluate(async () => {
      const root = document.getElementById("root");
      const stack = [root[Object.keys(root).find((key) => key.startsWith("__reactContainer"))].stateNode.current];
      while (stack.length) {
        const fiber = stack.pop();
        const client = fiber.memoizedProps?.client;
        if (typeof client?.invalidateQueries === "function") {
          await client.invalidateQueries({ queryKey: ["autopilot-templates"] });
          return;
        }
        if (fiber.child) stack.push(fiber.child);
        if (fiber.sibling) stack.push(fiber.sibling);
      }
      throw new Error("Live QueryClient not found");
    });
    await page.waitForTimeout(100);
    await expect(deliberateOwner).toBeFocused();
    results.refetch.push({ language, origin, deliberateFocusRetained: true });
    await save();
  }
}

await locale("zh-Hans");
const names = JSON.parse(await fs.readFile(path.join(process.cwd(), "packages/views/locales/zh-Hans/autopilots.json"), "utf8"));
const touch = async (locator) => {
  await locator.scrollIntoViewIfNeeded();
  const box = await locator.boundingBox();
  const x = box.x + box.width / 2;
  const y = box.y + box.height / 2;
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x, y, radiusX: 1, radiusY: 1, force: 1 }] });
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
};
await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 1 });
expect(await page.evaluate(() => matchMedia("(pointer: coarse)").matches)).toBe(true);
for (const origin of ["empty-list", "creation-gallery"]) {
  for (const width of [1771, 390]) {
    await control("/viewport", { width, height: width === 390 ? 844 : 1080, zoom: 1 });
    await navigate("push", origin === "empty-list" ? listPath : pickerPath);
    await finalRoute(origin, names);
    const filter = root().getByRole("button", { name: "仓库维护 4", exact: true });
    await touch(filter);
    await expect(filter).toHaveAttribute("aria-pressed", "true");
    await expect(root().locator("li")).toHaveCount(4);
    const blank = root().getByRole("button", { name: "从空白开始", exact: true });
    const blankBox = await blank.boundingBox();
    const filterBox = await filter.boundingBox();
    expect(blankBox.height).toBeGreaterThanOrEqual(44);
    expect(filterBox.height).toBeGreaterThanOrEqual(44);
    await touch(blank);
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(page.getByRole("dialog")).toHaveCSS("opacity", "1");
    await page.getByRole("dialog").evaluate(async () => {
      await Promise.all(document.getAnimations().filter((animation) => animation.effect?.getComputedTiming().endTime !== Infinity).map((animation) => animation.finished.catch(() => {})));
    });
    const screenshot = await shot(`zh-${origin}-blank-touch-${width}`);
    await page.keyboard.press("Escape");
    await expect(page.getByRole("dialog")).toHaveCount(0);
    await expect(blank).toBeFocused();
    results.touch.push({ origin, width, pointer: "coarse", engine: "Chromium Input.dispatchTouchEvent", blankHeight: blankBox.height, filterHeight: filterBox.height, filterCount: 4, dialogOpened: true, focusReturnedOnDismiss: true, screenshot: path.basename(screenshot) });
    await save();
    console.log(JSON.stringify({ origin, width, touch: "pass", blankHeight: blankBox.height }));
  }
}
await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
await control("/viewport", { width: 1771, height: 1080, zoom: 1 });
await navigate("push", listPath);
await finalRoute("empty-list", names);
await root().getByRole("button", { name: "全部 10", exact: true }).click();
await root().evaluate((node) => { node.scrollTop = 0; document.activeElement?.blur(); });
results.runtime = await control("/status");
expect(results.runtime.counters.daemonStarts).toBe(0);
expect(results.runtime.counters.installCalls).toBe(0);
expect(results.runtime.counters.externalLinks).toBe(0);
expect(results.runtime.counters.blockedWrites).toBe(0);
expect(results.runtime.pageErrors).toEqual([]);
await save();
