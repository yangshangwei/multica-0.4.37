import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { SupportedLocale } from "@multica/core/i18n";
import { RESOURCES } from "@multica/views/locales";
import { copyText } from "@multica/ui/lib/clipboard";
import { toast } from "sonner";
import { DaemonPanel } from "./daemon-panel";

vi.mock("@multica/ui/lib/clipboard", () => ({ copyText: vi.fn() }));
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

let receiveLog: (line: string) => void;
const onOpenChange = vi.fn();

function panel(locale: SupportedLocale = "en", runtimeCount = 1) {
  return (
    <I18nProvider locale={locale} resources={RESOURCES}>
      <DaemonPanel
        open
        onOpenChange={onOpenChange}
        status={{ state: "running", uptime: "1h2m" }}
        runtimeCount={runtimeCount}
      />
    </I18nProvider>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(copyText).mockResolvedValue(true);
  Object.defineProperty(window, "daemonAPI", {
    configurable: true,
    value: {
      startLogStream: vi.fn(),
      stopLogStream: vi.fn(),
      onLogLine: vi.fn((callback: (line: string) => void) => {
        receiveLog = callback;
        return vi.fn();
      }),
    },
  });
});

describe("DaemonPanel language selection", () => {
  it("switches controls, status and counts without restarting the log stream", async () => {
    const { rerender } = render(panel());
    expect(screen.getByRole("heading", { name: "Local daemon logs" })).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
    expect(screen.getByText("· 1 runtime")).toBeInTheDocument();

    rerender(panel("zh-Hans", 2));
    expect(await screen.findByRole("heading", { name: "本地守护进程日志" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "复制" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "清空" })).toBeDisabled();
    expect(screen.getByPlaceholderText("搜索…")).toBeInTheDocument();
    expect(screen.getByText("运行中")).toBeInTheDocument();
    expect(screen.getByText("· 1 小时 2 分钟")).toBeInTheDocument();
    expect(screen.getByText("· 2 个运行时")).toBeInTheDocument();
    expect(screen.getByText("显示 0 行，共 0 行")).toBeInTheDocument();
    expect(window.daemonAPI.startLogStream).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);

    rerender(panel());
    expect(await screen.findByRole("button", { name: "Copy" })).toBeInTheDocument();
  });

  it("localizes repeated groups and copy feedback while preserving raw log lines", async () => {
    const { rerender } = render(panel());
    const first = "12:00:00.000 INF task completed component=daemon";
    const second = "12:00:01.000 INF task completed component=daemon";
    act(() => receiveLog(first));
    fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Copied 1 line"));
    act(() => receiveLog(second));
    fireEvent.click(screen.getByRole("button", { name: "Copy" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Copied 2 lines"));
    expect(copyText).toHaveBeenLastCalledWith(`${first}\n${second}`);

    rerender(panel("zh-Hans"));
    const expand = await screen.findByRole("button", {
      name: /还有 1 条“task completed”，点击展开/,
    });
    expect(screen.getByText("task completed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "INFO2" })).toBeInTheDocument();
    fireEvent.click(expand);
    expect(screen.getByRole("button", { name: /收起 2 条重复日志/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "复制" }));
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("已复制 2 行日志"));
    expect(copyText).toHaveBeenLastCalledWith(`${first}\n${second}`);

    vi.mocked(copyText).mockResolvedValue(false);
    fireEvent.click(screen.getByRole("button", { name: "复制" }));
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("复制失败"));
    fireEvent.click(screen.getByRole("button", { name: "清空" }));
    expect(screen.getByText("显示 0 行，共 0 行")).toBeInTheDocument();
  });

  it("localizes the search empty state", () => {
    render(panel("zh-Hans"));
    act(() => receiveLog("12:00:00.000 INF task completed"));
    fireEvent.change(screen.getByPlaceholderText("搜索…"), {
      target: { value: "not found" },
    });
    expect(screen.getByText("没有匹配的日志")).toBeInTheDocument();
    expect(screen.getByText("试试其他搜索词或调整日志级别筛选。")).toBeInTheDocument();
  });
});
