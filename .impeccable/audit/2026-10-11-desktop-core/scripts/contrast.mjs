export async function scanContrast(page) {
  return page.evaluate(() => {
    const canvas = document.createElement('canvas'); canvas.width = canvas.height = 1;
    const ctx = canvas.getContext('2d', { willReadFrequently: true });
    const rgb = color => {
      ctx.clearRect(0, 0, 1, 1); ctx.fillStyle = color; ctx.fillRect(0, 0, 1, 1);
      return [...ctx.getImageData(0, 0, 1, 1).data].map((v, i) => i === 3 ? v / 255 : v);
    };
    const blend = (a, b) => a.slice(0, 3).map((v, i) => v * a[3] + b[i] * (1 - a[3])).concat(1);
    const luminance = color => color.slice(0, 3).map(v => v / 255).map(v => v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4).reduce((n, v, i) => n + v * [0.2126, 0.7152, 0.0722][i], 0);
    const checked = [];
    for (const el of document.body.querySelectorAll('*')) {
      if (!(el instanceof HTMLElement) || el.closest('[inert], [aria-hidden="true"], [disabled], .sr-only')) continue;
      const text = [...el.childNodes].filter(n => n.nodeType === 3).map(n => n.textContent.trim()).join(' ').trim();
      if (!text || el.tagName === 'SCRIPT' || el.tagName === 'STYLE') continue;
      const rect = el.getBoundingClientRect();
      if (!rect.width || !rect.height || rect.bottom < 0 || rect.top > innerHeight || rect.right < 0 || rect.left > innerWidth) continue;
      const chain = []; let node = el; let excluded = false;
      while (node) {
        const s = getComputedStyle(node);
        if (Number(s.opacity) < 1 || s.visibility === 'hidden' || s.backgroundImage !== 'none') { excluded = true; break; }
        chain.unshift(s.backgroundColor); node = node.parentElement;
      }
      if (excluded) continue;
      let bg = [255, 255, 255, 1]; for (const color of chain) bg = blend(rgb(color), bg);
      const style = getComputedStyle(el); const fg = blend(rgb(style.color), bg);
      const a = luminance(fg), b = luminance(bg); const ratio = (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
      const size = parseFloat(style.fontSize), bold = parseInt(style.fontWeight) >= 700;
      const floor = size >= 24 || (size >= 18.66 && bold) ? 3 : 4.5;
      checked.push({ text: text.slice(0, 100), class: el.className, color: style.color, background: bg, fontSize: size, ratio: Number(ratio.toFixed(2)), floor });
    }
    return { measurements: checked, checked: checked.length, failures: checked.filter(x => x.ratio < x.floor), samples: checked.filter(x => /text-muted-foreground/.test(x.class)).slice(0, 5) };
  });
}
