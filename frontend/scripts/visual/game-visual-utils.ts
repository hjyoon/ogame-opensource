import type { Browser, Page } from "@playwright/test";
import type { GameVisualAction, GameVisualScreenSpec, LayoutBoxName, SideName } from "./game-screen-registry";

export type Box = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type DiffResult = {
  width: number;
  height: number;
  totalPixels: number;
  changedPixels: number;
  diffRatio: number;
  averageDelta: number;
};

export type PageCaptureContract = {
  consoleErrors: string[];
  failedRequests: string[];
  badResponses: string[];
  boxes: Record<string, Box | null>;
  textChecks: Record<string, boolean>;
};

export const deterministicScreenshotCSS = `
*, *::before, *::after {
  animation-delay: 0s !important;
  animation-duration: 0s !important;
  animation-iteration-count: 1 !important;
  caret-color: transparent !important;
  scroll-behavior: auto !important;
  transition-delay: 0s !important;
  transition-duration: 0s !important;
}
html, body {
  overflow-anchor: none !important;
}
`;

export async function waitForImages(page: Page): Promise<void> {
  await page.evaluate(async () => {
    await Promise.all(
      Array.from(document.images).map(async (image) => {
        try {
          await image.decode();
        } catch {
          // Broken optional icons are reported through request tracking.
        }
      })
    );
  });
}

export async function waitForStablePaint(page: Page): Promise<void> {
  await page.evaluate(async () => {
    if ("fonts" in document) {
      await document.fonts.ready;
    }
    await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
  });
}

export async function performVisualActions(page: Page, side: SideName, actions: GameVisualAction[] = []): Promise<void> {
  for (const action of actions) {
    const selector = actionSelector(action, side);
    if (!selector) {
      continue;
    }
    const locator = page.locator(selector).first();
    if ((await locator.count()) === 0) {
      continue;
    }
    if (action.type === "hover") {
      await locator.hover({ timeout: 5_000 });
    } else if (action.type === "focus") {
      await locator.focus({ timeout: 5_000 });
    } else if (action.type === "click") {
      await locator.click({ timeout: 5_000 });
    } else if (action.type === "fill") {
      await locator.fill(action.value ?? "", { timeout: 5_000 });
    } else if (action.type === "select") {
      await locator.selectOption(action.value ?? "", { timeout: 5_000 });
    } else if (action.type === "check") {
      await locator.check({ timeout: 5_000 });
    } else if (action.type === "uncheck") {
      await locator.uncheck({ timeout: 5_000 });
    } else {
      await locator.press(action.value ?? "Tab", { timeout: 5_000 });
    }
    if (action.waitForSelector) {
      await page.locator(action.waitForSelector).first().waitFor({ timeout: 5_000 });
    }
    if (action.waitMs && action.waitMs > 0) {
      await page.waitForTimeout(action.waitMs);
    }
  }
}

export async function normalizeDynamicPageParts(page: Page, side: SideName, spec: GameVisualScreenSpec, maskSelectors: string[]): Promise<void> {
  const tooltipVisible = spec.area === "hover";
  await page.evaluate(
    ({ currentPageName, pageSide, selectors, keepTooltips }) => {
      if (document.activeElement instanceof HTMLElement) {
        document.activeElement.blur();
      }
      const hide = (selector: string) => {
        for (const element of document.querySelectorAll(selector)) {
          if (element instanceof HTMLElement) {
            element.style.visibility = "hidden";
          }
        }
      };
      const makeTextTransparent = (element: HTMLElement) => {
        element.style.color = "transparent";
        element.style.textDecorationColor = "transparent";
        for (const child of element.querySelectorAll<HTMLElement>("*")) {
          if (keepTooltips && child.closest(".legacy-galaxy-tooltip, #overDiv")) {
            continue;
          }
          child.style.color = "transparent";
          child.style.textDecorationColor = "transparent";
        }
      };
      const replaceNativeCheckboxes = (selector: string) => {
        for (const checkbox of document.querySelectorAll<HTMLInputElement>(selector)) {
          const marker = document.createElement("span");
          marker.setAttribute("data-visual-checkbox", checkbox.checked ? "checked" : "unchecked");
          marker.textContent = checkbox.checked ? "\u2713" : "";
          marker.style.background = checkbox.checked ? "#1a73e8" : "#ffffff";
          marker.style.border = "1px solid #9aa9bd";
          marker.style.boxSizing = "border-box";
          marker.style.color = "#ffffff";
          marker.style.display = "inline-block";
          marker.style.fontFamily = "Arial, sans-serif";
          marker.style.fontSize = "11px";
          marker.style.height = "13px";
          marker.style.lineHeight = "11px";
          marker.style.margin = getComputedStyle(checkbox).margin;
          marker.style.textAlign = "center";
          marker.style.verticalAlign = "middle";
          marker.style.width = "13px";
          checkbox.replaceWith(marker);
        }
      };

      for (const selector of selectors) {
        if (keepTooltips && (selector.includes("tooltip") || selector === "#overDiv")) {
          continue;
        }
        hide(selector);
      }
      if (pageSide === "legacy" && !keepTooltips) {
        hide("#overDiv");
      }
      hide("#header_top, .legacy-header-top, #resources, .legacy-resource-table, select[name='cp']");

      const resourceValues = Array.from(document.querySelectorAll<HTMLTableCellElement>("#resources tr:nth-child(3) td"));
      const normalizedResourceValues = ["000.000", "000.000", "0.000", "0", "0/0"];
      resourceValues.forEach((cell, index) => {
        cell.textContent = normalizedResourceValues[index] ?? "0";
      });

      if (currentPageName.startsWith("game-overview") || currentPageName === "game-empire-redirect") {
        hide("#content img[width='50'][height='50'], .legacy-overview-main-table img[width='50'][height='50']");
        for (const headerCell of document.querySelectorAll<HTMLTableCellElement>(".legacy-overview-main-table th, #content table th")) {
          if (headerCell.textContent?.trim() === "Server time") {
            const timeCell = headerCell.nextElementSibling;
            if (timeCell instanceof HTMLElement) {
              timeCell.textContent = "Fri Jun 19 00:00:00";
            }
          }
        }
        for (const eventCell of document.querySelectorAll<HTMLElement>(".legacy-overview-main-table td, .legacy-overview-main-table th, #content table td, #content table th")) {
          const text = eventCell.textContent ?? "";
          if (text.includes("Mission:") && (text.includes("has been sent") || text.includes("returns"))) {
            makeTextTransparent(eventCell);
          }
        }
        for (const row of document.querySelectorAll<HTMLTableRowElement>(".legacy-overview-main-table tr, #content table tr")) {
          const cells = Array.from(row.querySelectorAll<HTMLElement>("th, td"));
          if (cells[0]?.textContent?.trim() === "Position" && cells[1]) {
            cells[1].textContent = "[0:0:0]";
          }
          if (cells[0]?.textContent?.trim() === "Points" && cells[1]) {
            cells[1].textContent = "0 (Rank 0 of 1.066)";
          }
        }
      }

      for (const countdown of document.querySelectorAll<HTMLElement>("[id^='bxx'], .legacy-admin-queue-countdown")) {
        countdown.textContent = "0:00:00";
        countdown.setAttribute("title", "0");
      }

      if (currentPageName === "game-empire-redirect") {
        hide("#content img[width='200'][height='200'], .legacy-overview-table img[width='200'][height='200']");
      }
      if (currentPageName === "game-admin-db") {
        for (const row of document.querySelectorAll<HTMLTableRowElement>("#content tr, .legacy-admin-content tr")) {
          if (/backup_.*\.json|Restore Delete/.test(row.textContent ?? "")) {
            row.remove();
          }
        }
      }
      if (currentPageName.startsWith("game-admin-")) {
        for (const image of document.querySelectorAll<HTMLImageElement>("img")) {
          const box = image.getBoundingClientRect();
          if (box.top < 85 && box.left > 150) {
            image.style.visibility = "hidden";
          }
        }
      }
      if (currentPageName === "game-admin-battlesim") {
        hide("#content input, #content select, #content textarea, .legacy-admin-content input, .legacy-admin-content select, .legacy-admin-content textarea");
        for (const cell of document.querySelectorAll<HTMLTableCellElement>("#content td, .legacy-admin-content td")) {
          if (cell.textContent?.trim().startsWith("Slot:")) {
            cell.innerHTML = "&nbsp;";
          }
        }
      }
      if (currentPageName === "game-officers") {
        hide("#content img[src$='DMaterie.jpg'], .legacy-officers-table img[src$='DMaterie.jpg']");
      }
      if (currentPageName === "game-admin-fleetlogs") {
        for (const cell of document.querySelectorAll<HTMLElement>("#content table th, #content table td, .legacy-admin-fleetlogs-table th, .legacy-admin-fleetlogs-table td")) {
          cell.style.color = "transparent";
          cell.style.borderColor = "transparent";
          for (const child of cell.querySelectorAll<HTMLElement>("*")) {
            child.style.color = "transparent";
          }
        }
        for (const input of document.querySelectorAll<HTMLElement>("#content table input, .legacy-admin-fleetlogs-table input")) {
          input.style.visibility = "hidden";
        }
        for (const nestedTable of document.querySelectorAll<HTMLElement>("#content table table, .legacy-admin-fleetlogs-table table")) {
          nestedTable.style.visibility = "hidden";
        }
        for (const table of document.querySelectorAll<HTMLElement>("#content table, .legacy-admin-fleetlogs-table")) {
          if (table.textContent?.includes("Timer") && table.textContent.includes("Command")) {
            table.style.visibility = "hidden";
          }
        }
      }
      if (currentPageName.startsWith("game-galaxy")) {
        const galaxyTables = Array.from(document.querySelectorAll<HTMLElement>("#content table, .legacy-galaxy-table")).filter((table) => {
          const text = table.textContent ?? "";
          return text.includes("Solar system") && text.includes("Far space") && text.includes("Legend");
        });
        for (const table of galaxyTables) {
          for (const cell of table.querySelectorAll<HTMLElement>("th, td")) {
            makeTextTransparent(cell);
          }
        }
        hide("#content img[src$='b.gif'], .legacy-galaxy-table img[src$='b.gif']");
        if (keepTooltips) {
          for (const table of galaxyTables) {
            table.style.visibility = "hidden";
          }
          for (const image of document.querySelectorAll<HTMLImageElement>("#content table img, .legacy-galaxy-table img")) {
            if (!image.closest(".legacy-galaxy-tooltip, #overDiv")) {
              image.style.visibility = "hidden";
            }
          }
          for (const tooltip of document.querySelectorAll<HTMLElement>(".legacy-galaxy-tooltip, .legacy-galaxy-tooltip *, #overDiv, #overDiv *")) {
            tooltip.style.color = "";
            tooltip.style.textDecorationColor = "";
            tooltip.style.visibility = "visible";
          }
        }
      }
      if (currentPageName === "game-admin-queue") {
        for (const cell of document.querySelectorAll<HTMLElement>("#content table th, #content table td, .legacy-admin-queue-table th, .legacy-admin-queue-table td")) {
          const text = cell.textContent ?? "";
          if (text.includes("ADM_QUEUE_FROZEN")) {
            cell.textContent = text.replace(/ADM_QUEUE_FROZEN\s+\d+/g, "ADM_QUEUE_FROZEN 000");
          }
        }
      }
      if (currentPageName === "game-admin-planets") {
        const dynamicLabels = new Set(["Creation date", "Date of removal", "Last activity", "Last state update"]);
        for (const row of document.querySelectorAll<HTMLTableRowElement>("#content tr, .legacy-admin-planets-detail tr")) {
          const cells = Array.from(row.querySelectorAll<HTMLElement>("th, td"));
          if (cells.length >= 2 && dynamicLabels.has(cells[0].textContent?.trim() ?? "")) {
            cells[1].textContent = "2026-06-19 00:00:00";
          }
        }
      }
      if (currentPageName.startsWith("game-statistics")) {
        for (const cell of document.querySelectorAll<HTMLTableCellElement>(".legacy-statistics-head-table td, #content table td")) {
          if (cell.textContent?.trim().startsWith("Statistics (as of:")) {
            cell.textContent = "Statistics (as of: 2026-06-19, 00:00:00)";
            break;
          }
        }
      }
      if (currentPageName === "game-alliance-ranks") {
        replaceNativeCheckboxes("#content input[type='checkbox'], .legacy-alliance-ranks-table input[type='checkbox']");
      }
      if (currentPageName.startsWith("game-options")) {
        replaceNativeCheckboxes("#content input[type='checkbox'], .legacy-options-table input[type='checkbox']");
      }
    },
    {
      currentPageName: spec.name,
      pageSide: side,
      selectors: maskSelectors,
      keepTooltips: tooltipVisible
    }
  );
}

export async function expectedTextChecks(page: Page, expectedTexts: string[]): Promise<Record<string, boolean>> {
  return await page.evaluate((texts) => {
    const bodyText = document.body.textContent ?? "";
    return Object.fromEntries(texts.map((text) => [text, bodyText.includes(text)]));
  }, expectedTexts);
}

export async function boxFor(page: Page, selector: string): Promise<Box | null> {
  const locator = page.locator(selector).first();
  if ((await locator.count()) === 0) {
    return null;
  }
  const box = await locator.boundingBox();
  return box ? { x: box.x, y: box.y, width: box.width, height: box.height } : null;
}

export async function compareScreenshots(
  browser: Browser,
  legacyPath: string,
  migratedPath: string,
  diffPath: string,
  colorDeltaThreshold: number
): Promise<DiffResult> {
  const page = await browser.newPage({ viewport: { width: 16, height: 16 } });
  const legacy = await Bun.file(legacyPath).arrayBuffer();
  const migrated = await Bun.file(migratedPath).arrayBuffer();
  const result = await page.evaluate(
    async ({ left, right, threshold }) => {
      const leftImage = await loadImage(left);
      const rightImage = await loadImage(right);
      const width = Math.min(leftImage.width, rightImage.width);
      const height = Math.min(leftImage.height, rightImage.height);
      const canvas = document.createElement("canvas");
      canvas.width = width;
      canvas.height = height;
      const ctx = canvas.getContext("2d", { willReadFrequently: true });
      if (!ctx) {
        throw new Error("2D canvas is unavailable");
      }
      ctx.drawImage(leftImage, 0, 0);
      const leftPixels = ctx.getImageData(0, 0, width, height).data;
      ctx.clearRect(0, 0, width, height);
      ctx.drawImage(rightImage, 0, 0);
      const rightPixels = ctx.getImageData(0, 0, width, height).data;
      const diffImage = ctx.createImageData(width, height);
      let changedPixels = 0;
      let totalDelta = 0;
      for (let i = 0; i < leftPixels.length; i += 4) {
        const delta =
          Math.abs(leftPixels[i] - rightPixels[i]) +
          Math.abs(leftPixels[i + 1] - rightPixels[i + 1]) +
          Math.abs(leftPixels[i + 2] - rightPixels[i + 2]) +
          Math.abs(leftPixels[i + 3] - rightPixels[i + 3]);
        totalDelta += delta / 4;
        if (delta / 4 > threshold) {
          changedPixels += 1;
          diffImage.data[i] = 255;
          diffImage.data[i + 1] = 0;
          diffImage.data[i + 2] = 0;
          diffImage.data[i + 3] = 255;
        } else {
          const faded =
            230 +
            Math.round(
              (0.2126 * leftPixels[i] + 0.7152 * leftPixels[i + 1] + 0.0722 * leftPixels[i + 2]) * 0.1
            );
          diffImage.data[i] = faded;
          diffImage.data[i + 1] = faded;
          diffImage.data[i + 2] = faded;
          diffImage.data[i + 3] = 255;
        }
      }
      const totalPixels = width * height;
      ctx.putImageData(diffImage, 0, 0);
      return {
        width,
        height,
        totalPixels,
        changedPixels,
        diffRatio: changedPixels / totalPixels,
        averageDelta: totalDelta / totalPixels,
        diffDataURL: canvas.toDataURL("image/png")
      };

      async function loadImage(dataUrl: string): Promise<HTMLImageElement> {
        const image = new Image();
        image.src = dataUrl;
        await image.decode();
        return image;
      }
    },
    {
      left: `data:image/png;base64,${Buffer.from(legacy).toString("base64")}`,
      right: `data:image/png;base64,${Buffer.from(migrated).toString("base64")}`,
      threshold: colorDeltaThreshold
    }
  );
  await page.close();
  const base64 = result.diffDataURL.replace(/^data:image\/png;base64,/, "");
  await Bun.write(diffPath, Uint8Array.from(atob(base64), (char) => char.charCodeAt(0)));
  return {
    width: result.width,
    height: result.height,
    totalPixels: result.totalPixels,
    changedPixels: result.changedPixels,
    diffRatio: result.diffRatio,
    averageDelta: result.averageDelta
  };
}

export function boxesPresent(boxes: Record<string, Box | null>, requiredBoxes: LayoutBoxName[] = ["header", "menu", "content"]): boolean {
  return requiredBoxes.every((boxName) => boxes[boxName] !== null);
}

export function maxPairBoxDelta(
  left: Record<string, Box | null>,
  right: Record<string, Box | null>,
  requiredBoxes: LayoutBoxName[] = ["header", "menu", "content"]
): number {
  let maxDelta = 0;
  for (const key of requiredBoxes) {
    const leftBox = left[key];
    const rightBox = right[key];
    if (!leftBox || !rightBox) {
      return Number.POSITIVE_INFINITY;
    }
    maxDelta = Math.max(
      maxDelta,
      Math.abs(leftBox.x - rightBox.x),
      Math.abs(leftBox.y - rightBox.y),
      Math.abs(leftBox.width - rightBox.width),
      Math.abs(leftBox.height - rightBox.height)
    );
  }
  return maxDelta;
}

export function textChecksEquivalent(legacy: Record<string, boolean>, migrated: Record<string, boolean>): boolean {
  return textCheckMismatches(legacy, migrated).length === 0;
}

export function textCheckMismatches(legacy: Record<string, boolean>, migrated: Record<string, boolean>): string[] {
  const texts = new Set([...Object.keys(legacy), ...Object.keys(migrated)]);
  return Array.from(texts)
    .filter((text) => legacy[text] !== migrated[text])
    .map((text) => `text parity mismatch: ${text} legacy=${legacy[text] === true} migrated=${migrated[text] === true}`);
}

export function caseNotes(legacy: PageCaptureContract, migrated: PageCaptureContract, diff: DiffResult, boxMaxDelta: number): string[] {
  return [
    ...legacy.consoleErrors.map((value) => `legacy console: ${value}`),
    ...migrated.consoleErrors.map((value) => `migrated console: ${value}`),
    ...legacy.failedRequests.map((value) => `legacy failed: ${value}`),
    ...migrated.failedRequests.map((value) => `migrated failed: ${value}`),
    ...legacy.badResponses.map((value) => `legacy response: ${value}`),
    ...migrated.badResponses.map((value) => `migrated response: ${value}`),
    ...textCheckMismatches(legacy.textChecks, migrated.textChecks),
    `diff ratio ${formatNumber(diff.diffRatio)}`,
    `box max delta ${formatNumber(boxMaxDelta)}`
  ];
}

export function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}

export function numberEnv(name: string, fallback: number): number {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : fallback;
}

export function formatNumber(value: number): string {
  if (value === 0 || Number.isInteger(value)) {
    return String(value);
  }
  return value.toPrecision(12).replace(/\.?0+$/, "");
}

function actionSelector(action: GameVisualAction, side: SideName): string {
  return (side === "legacy" ? action.legacySelector : action.migratedSelector) ?? action.selector ?? "";
}
