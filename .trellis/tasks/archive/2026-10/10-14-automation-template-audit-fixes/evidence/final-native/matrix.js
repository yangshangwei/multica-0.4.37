const variants = [
  { name: "wide", width: 1771, height: 1080, zoom: 1 },
  { name: "compact", width: 886, height: 720, zoom: 1 },
  { name: "narrow", width: 390, height: 844, zoom: 1 },
  { name: "zoom200", width: 1771, height: 1080, zoom: 2 },
];
const results = [];
for (const language of ["zh-Hans", "en"]) {
  await locale(language);
  for (const appearance of ["light", "dark"]) {
    await theme(appearance);
    for (const variant of variants) {
      await cdp.send("Emulation.clearDeviceMetricsOverride");
      await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: false });
      await control("/viewport", variant);
      await expect(root().locator("li")).toHaveCount(10);
      await root().evaluate((node) => { node.scrollTop = 0; document.activeElement?.blur(); });
      await page.mouse.move(1, 60);
      await page.waitForTimeout(350);
      const name = `${language === "en" ? "en" : "zh"}-${appearance}-${variant.name}`;
      const screenshot = await shot(name);
      const layout = await root().evaluate((node) => {
        const box = (element) => {
          const { x, y, width, height } = element.getBoundingClientRect();
          return { x, y, width, height };
        };
        const grid = node.querySelector("ul");
        const buttons = [...node.querySelectorAll("button")];
        const blank = buttons[0];
        return {
          viewport: { width: innerWidth, height: innerHeight, dpr: devicePixelRatio },
          catalog: box(node),
          columns: getComputedStyle(grid).gridTemplateColumns,
          horizontalOverflow: node.scrollWidth > node.clientWidth || document.documentElement.scrollWidth > innerWidth,
          intro: { text: node.querySelector("p").textContent, ...box(node.querySelector("p")) },
          blank: { text: blank.innerText, ...box(blank) },
          filters: buttons.filter((button) => button.hasAttribute("aria-pressed")).map((button) => ({ text: button.innerText, pressed: button.getAttribute("aria-pressed"), ...box(button) })),
          cards: [...grid.querySelectorAll("li > button")].map((card) => ({
            title: card.querySelector(".text-title-sm").textContent,
            ...box(card), overflow: card.scrollWidth > card.clientWidth,
          })),
        };
      });
      expect(layout.horizontalOverflow, name).toBe(false);
      expect(layout.cards.every((card) => !card.overflow), name).toBe(true);
      expect(layout.intro.y + layout.intro.height <= layout.cards[0].y, name).toBe(true);
      expect(layout.blank.text.trim().length, name).toBeGreaterThan(0);
      if (variant.name === "narrow") expect(layout.blank.height, name).toBeGreaterThanOrEqual(44);
      await root().getByRole("group").getByRole("button").last().focus();
      await page.keyboard.press("Tab");
      const firstCard = root().locator("li > button").first();
      await expect(firstCard).toBeFocused();
      const focus = await firstCard.evaluate((card) => {
        const canvas = document.createElement("canvas");
        canvas.width = canvas.height = 1;
        const ctx = canvas.getContext("2d", { willReadFrequently: true });
        const rgba = (color) => {
          ctx.clearRect(0, 0, 1, 1);
          ctx.fillStyle = color;
          ctx.fillRect(0, 0, 1, 1);
          return [...ctx.getImageData(0, 0, 1, 1).data].map((value, index) => index === 3 ? value / 255 : value);
        };
        const background = (element) => {
          const layers = [];
          for (let current = element; current; current = current.parentElement) layers.push(rgba(getComputedStyle(current).backgroundColor));
          return layers.reverse().reduce((below, color) => color.slice(0, 3).map((value, index) => value * color[3] + below[index] * (1 - color[3])), [255, 255, 255]);
        };
        const luminance = (color) => color.slice(0, 3).map((value) => {
          const channel = value / 255;
          return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
        }).reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0);
        const ratio = (a, b) => (Math.max(luminance(a), luminance(b)) + 0.05) / (Math.min(luminance(a), luminance(b)) + 0.05);
        const style = getComputedStyle(card);
        const ring = style.getPropertyValue("--tw-ring-color").trim();
        const ringColor = rgba(ring);
        const surface = background(card);
        const canvasColor = background(card.parentElement);
        const intro = card.closest("section").querySelector("p");
        const text = rgba(getComputedStyle(intro).color);
        return {
          active: document.activeElement === card, visible: card.matches(":focus-visible"),
          ring, ringRGB: ringColor, cardRGB: surface, canvasRGB: canvasColor, shadow: style.boxShadow,
          contrastOnCard: ratio(ringColor, surface), contrastOnCanvas: ratio(ringColor, canvasColor),
          bodyContrast: ratio(text, background(intro)),
        };
      });
      expect(focus.visible, name).toBe(true);
      expect(Math.min(focus.contrastOnCard, focus.contrastOnCanvas), name).toBeGreaterThanOrEqual(3);
      expect(focus.bodyContrast, name).toBeGreaterThanOrEqual(4.5);
      const focusScreenshot = variant.name === "wide" ? await shot(`${name}-keyboard-focus`) : null;
      const native = await control("/status");
      expect(native.zoom, name).toBe(variant.zoom);
      results.push({ name, screenshot: path.basename(screenshot), focusScreenshot: focusScreenshot && path.basename(focusScreenshot), nativeZoom: native.zoom, nativeBounds: native.bounds, ...layout, focus });
      await fs.writeFile(path.join(dir, "matrix.json"), JSON.stringify(results, null, 2) + "\n");
      console.log(JSON.stringify({ name, columns: layout.columns, overflow: layout.horizontalOverflow, minFocusContrast: Math.min(focus.contrastOnCard, focus.contrastOnCanvas), bodyContrast: focus.bodyContrast, nativeZoom: native.zoom }));
    }
  }
}
await control("/viewport", variants[0]);
await locale("zh-Hans");
await theme("light");
await root().evaluate((node) => { node.scrollTop = 0; document.activeElement?.blur(); });
