import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DesktopEndpointSetupPage } from "./endpoint-setup";

afterEach(cleanup);

describe("DesktopEndpointSetupPage locale", () => {
  it.each(["en-US", "ja-JP", "ko-KR"])("uses English connection controls for %s", (systemLocale) => {
    const testRuntimeConfig = vi.fn().mockResolvedValue({ ok: true, latencyMs: 3 });
    Object.defineProperty(window, "desktopAPI", {
      configurable: true,
      value: { systemLocale, testRuntimeConfig },
    });
    render(<DesktopEndpointSetupPage initialApiUrl="http://localhost:18080" />);
    expect(screen.getByRole("heading", { name: "Connect to your Multica server" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
    expect(testRuntimeConfig).toHaveBeenCalledWith("http://localhost:18080");
  });

  it("retains Chinese connection controls and invalid-address protection", () => {
    const testRuntimeConfig = vi.fn();
    Object.defineProperty(window, "desktopAPI", {
      configurable: true,
      value: { systemLocale: "zh-CN", testRuntimeConfig },
    });
    render(<DesktopEndpointSetupPage initialApiUrl="file:///tmp/server" />);
    expect(screen.getByRole("heading", { name: "连接到 Multica 服务器" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "测试连接" }));
    expect(testRuntimeConfig).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "保存并继续" })).toBeDisabled();
  });
});
