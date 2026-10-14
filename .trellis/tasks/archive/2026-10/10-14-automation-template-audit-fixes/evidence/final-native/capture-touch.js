await locale("zh-Hans");
await theme("light");
await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true, maxTouchPoints: 1 });
const listPath = (await navigate("current")).split("/autopilots")[0] + "/autopilots";
const results = JSON.parse(await fs.readFile(path.join(dir, "interactions.json"), "utf8"));
for (const entry of results.touch) {
  await control("/viewport", { width: entry.width, height: entry.width === 390 ? 844 : 1080, zoom: 1 });
  await navigate("push", entry.origin === "empty-list" ? listPath : listPath + "/new/template");
  await expect(root()).toBeVisible();
  const blank = root().getByRole("button", { name: "从空白开始", exact: true });
  await blank.scrollIntoViewIfNeeded();
  const box = await blank.boundingBox();
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: box.x + box.width / 2, y: box.y + box.height / 2, radiusX: 1, radiusY: 1, force: 1 }] });
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCSS("opacity", "1");
  await page.getByRole("dialog").evaluate(async () => {
    await Promise.all(document.getAnimations().filter((animation) => animation.effect?.getComputedTiming().endTime !== Infinity).map((animation) => animation.finished.catch(() => {})));
  });
  await shot(entry.screenshot.replace(/\.png$/, ""));
  entry.captureAfterOpeningAnimation = true;
  entry.dialogOpacity = await page.getByRole("dialog").evaluate((node) => getComputedStyle(node).opacity);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(blank).toBeFocused();
  console.log(JSON.stringify({ origin: entry.origin, width: entry.width, stableOpacity: entry.dialogOpacity }));
}
await fs.writeFile(path.join(dir, "interactions.json"), JSON.stringify(results, null, 2) + "\n");
await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
await control("/viewport", { width: 1771, height: 1080, zoom: 1 });
await navigate("push", listPath);
await expect(root()).toBeVisible();
