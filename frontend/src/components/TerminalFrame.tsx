import { Button, Card, Space, Typography } from "antd";
import { useEffect, useRef, useState } from "react";
import { Terminal } from "xterm";
import { FitAddon } from "xterm-addon-fit";
import "xterm/css/xterm.css";

type TerminalDisplayStatus = "CONNECTING" | "CONNECTED" | "DISCONNECTED";

type TerminalFrameProps = {
  title: string;
  lines: string[];
  status: "CONNECTED" | "DISCONNECTED";
  wsUrl: string;
  allowReconnect?: boolean;
  onStatusChange?: (status: TerminalDisplayStatus) => void;
  onReconnect?: () => void;
};

export function TerminalFrame({
  title,
  lines,
  status,
  wsUrl,
  allowReconnect = false,
  onStatusChange,
  onReconnect,
}: TerminalFrameProps) {
  const terminalRef = useRef<HTMLDivElement | null>(null);
  const xtermRef = useRef<Terminal | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const [socketStatus, setSocketStatus] = useState<TerminalDisplayStatus>("CONNECTING");
  const [lastError, setLastError] = useState<string | null>(null);
  const [reconnectSeed, setReconnectSeed] = useState(0);

  useEffect(() => {
    onStatusChange?.(socketStatus);
  }, [onStatusChange, socketStatus]);

  useEffect(() => {
    if (!terminalRef.current) {
      return;
    }

    terminalRef.current.innerHTML = "";
    setSocketStatus("CONNECTING");
    setLastError(null);

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
    terminal.writeln("AegisOps WebSSH session");
    terminal.writeln("----------------------------------------");
    lines.forEach((line) => terminal.writeln(line));
    terminal.writeln("");

    const socket = new WebSocket(wsUrl);
    socketRef.current = socket;
    socket.onopen = () => {
      setSocketStatus("CONNECTED");
      terminal.writeln("Connected.");
    };
    socket.onmessage = (event) => {
      terminal.write(String(event.data));
    };
    socket.onerror = () => {
      setLastError("WebSocket 建连失败，请检查会话状态或稍后重试。");
      terminal.writeln("\r\nWebSSH connection failed.");
    };
    socket.onclose = () => {
      setSocketStatus("DISCONNECTED");
      terminal.writeln("\r\nDisconnected.");
    };

    const disposable = terminal.onData((data) => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(data);
      }
    });

    xtermRef.current = terminal;
    const resize = () => fitAddon.fit();
    window.addEventListener("resize", resize);
    return () => {
      window.removeEventListener("resize", resize);
      disposable.dispose();
      if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
        socket.close();
      }
      socketRef.current = null;
      terminal.dispose();
      xtermRef.current = null;
    };
  }, [lines, reconnectSeed, wsUrl]);

  const displayStatus = socketStatus === "CONNECTED" ? "CONNECTED" : status === "CONNECTED" ? socketStatus : status;

  return (
    <Card className="page-card">
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        <div className="page-toolbar">
          <Space direction="vertical" size={4}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {title}
            </Typography.Title>
            <Typography.Text type="secondary">WebSSH over WebSocket</Typography.Text>
          </Space>
          <Space>
            {allowReconnect ? (
              <Button
                disabled={displayStatus === "CONNECTING"}
                onClick={() => {
                  onReconnect?.();
                  setReconnectSeed((value) => value + 1);
                }}
              >
                {displayStatus === "CONNECTED" ? "重连" : "重新连接"}
              </Button>
            ) : null}
            <Button disabled>{displayStatus === "CONNECTED" ? "已连接" : displayStatus === "CONNECTING" ? "连接中" : "已断开"}</Button>
          </Space>
        </div>
        {lastError ? <Typography.Text type="danger">{lastError}</Typography.Text> : null}
        <div className="terminal-surface">
          <div ref={terminalRef} style={{ width: "100%", height: 520 }} />
        </div>
      </Space>
    </Card>
  );
}
