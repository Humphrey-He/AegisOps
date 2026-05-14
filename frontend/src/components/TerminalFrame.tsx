import { Button, Card, Space, Typography } from "antd";
import { useEffect, useRef } from "react";
import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";
import "xterm/css/xterm.css";

type TerminalFrameProps = {
  title: string;
  lines: string[];
  status: "CONNECTED" | "DISCONNECTED";
};

export function TerminalFrame({ title, lines, status }: TerminalFrameProps) {
  const terminalRef = useRef<HTMLDivElement | null>(null);
  const xtermRef = useRef<Terminal | null>(null);

  useEffect(() => {
    if (!terminalRef.current || xtermRef.current) {
      return;
    }

    const terminal = new Terminal({
      convertEol: true,
      cursorBlink: true,
      rows: 24,
      theme: {
        background: "#0b1220",
        foreground: "#dbeafe",
      },
    });
    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.open(terminalRef.current);
    fitAddon.fit();
    terminal.writeln(`AegisOps WebSSH mock session`);
    terminal.writeln(`----------------------------------------`);
    lines.forEach((line) => terminal.writeln(line));
    terminal.writeln("");
    terminal.write("$ ");

    let command = "";
    terminal.onData((data) => {
      if (status !== "CONNECTED") {
        return;
      }
      if (data === "\r") {
        const input = command.trim();
        if (input === "help") {
          terminal.writeln("\r\nAvailable demo commands: help, clear, status, exit");
        } else if (input === "clear") {
          terminal.clear();
        } else if (input === "status") {
          terminal.writeln("\r\nHost status: HEALTHY, WebSSH channel: CONNECTED");
        } else if (input === "exit") {
          terminal.writeln("\r\nSession remains open in MVP mock mode.");
        } else if (input) {
          terminal.writeln(`\r\nCommand not found: ${input}`);
        }
        command = "";
        terminal.write("\r\n$ ");
        return;
      }
      if (data === "\u007F") {
        if (command.length > 0) {
          command = command.slice(0, -1);
          terminal.write("\b \b");
        }
        return;
      }
      if (data >= " ") {
        command += data;
        terminal.write(data);
      }
    });

    xtermRef.current = terminal;
    const resize = () => fitAddon.fit();
    window.addEventListener("resize", resize);
    return () => {
      window.removeEventListener("resize", resize);
      terminal.dispose();
      xtermRef.current = null;
    };
  }, [lines, status]);

  return (
    <Card className="page-card">
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <div className="page-toolbar">
          <Space direction="vertical" size={4}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {title}
            </Typography.Title>
            <Typography.Text type="secondary">这是一期前端 demo，会话输入已接到 xterm.js 基础交互。</Typography.Text>
          </Space>
          <Button disabled>{status === "CONNECTED" ? "已连接" : "已断开"}</Button>
        </div>
        <div className="terminal-surface">
          <div ref={terminalRef} style={{ width: "100%", height: 520 }} />
        </div>
      </Space>
    </Card>
  );
}
