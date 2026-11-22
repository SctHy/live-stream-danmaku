import { useEffect, useRef, useState, type RefObject } from "react";
import { createRoomClient, type Message } from "./lib/roomClient";
import { mountOverlay } from "./danmaku";
import { playRoom } from "./player";
import { Settings } from "./Settings";

type Client = ReturnType<typeof createRoomClient>;
const params = new URLSearchParams(location.search);
const room = params.get("room") || "6";
const session = params.get("session") || "screening-01";
const numeric = /^\d+$/.test(room);

function Cursor({ client }: { client: RefObject<Client | null> }) {
  const [value, setValue] = useState("");
  useEffect(() => {
    const timer = setInterval(
      () => setValue(client.current?.cursor || ""),
      250,
    );
    return () => clearInterval(timer);
  }, [client]);
  return <span title={value}>游标 {value || "—"}</span>;
}

function Feed({
  client,
  feed,
}: {
  client: RefObject<Client | null>;
  feed: RefObject<{ rows: Map<string, Message>; changed: boolean }>;
}) {
  const [rows, setRows] = useState<Message[]>([]);
  const key = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const timer = setInterval(() => {
      if (!feed.current.changed || document.hidden) return;
      feed.current.changed = false;
      setRows(Array.from(feed.current.rows.values()).reverse());
    }, 100);
    return () => clearInterval(timer);
  }, [feed]);
  function moderate(action: "withdraw" | "mute", message: Message) {
    const field = key.current;
    if (!field) return;
    if (!field.value) {
      field.closest("details")!.open = true;
      field.focus();
      return;
    }
    void client.current?.moderate(action, message, field.value);
  }
  return (
    <aside>
      <h2>实时弹幕</h2>
      <ol id="messages">
        {rows.map((m) => (
          <li key={m.event_id}>
            <small>观众 {m.user_id}</small>
            <div className="actions">
              <button onClick={() => moderate("withdraw", m)}>撤回</button>
              <button onClick={() => moderate("mute", m)}>禁言 60s</button>
            </div>
            <span>{m.content}</span>
          </li>
        ))}
      </ol>
      {!rows.length && <p className="empty">暂无弹幕</p>}
      <details>
        <summary>房管密钥</summary>
        <input
          ref={key}
          type="password"
          aria-label="房管密钥"
          autoComplete="off"
        />
      </details>
    </aside>
  );
}

export function App() {
  const video = useRef<HTMLVideoElement>(null),
    canvas = useRef<HTMLCanvasElement>(null),
    stage = useRef<HTMLDivElement>(null);
  const client = useRef<Client | null>(null),
    overlay = useRef<ReturnType<typeof mountOverlay> | null>(null);
  const feed = useRef({ rows: new Map<string, Message>(), changed: false });
  const [source, setSource] = useState(numeric ? "连接直播源…" : "纯弹幕房");
  const [connection, setConnection] = useState("连接中"),
    [protocol, setProtocol] = useState("");
  const [stats, setStats] = useState("在线 — · 赞 —"),
    [feedback, setFeedback] = useState("");
  const [ready, setReady] = useState(false),
    [pending, setPending] = useState(false);
  const [draft, setDraft] = useState("");
  useEffect(() => {
    const engine = mountOverlay(canvas.current!);
    overlay.current = engine;
    const player = numeric
      ? playRoom(video.current!, room, setSource)
      : undefined;
    const buffer = feed.current;
    const transport = createRoomClient(room, session, {
      show(message, animate) {
        if (buffer.rows.has(message.event_id)) return;
        if (animate) engine.enqueue(message);
        if (document.hidden) return;
        buffer.rows.set(message.event_id, message);
        if (buffer.rows.size > 100)
          buffer.rows.delete(buffer.rows.keys().next().value!);
        buffer.changed = true;
      },
      withdraw(id) {
        buffer.rows.delete(id);
        buffer.changed = true;
        engine.removeMessage(id);
      },
      clear() {
        buffer.rows.clear();
        buffer.changed = true;
        engine.resetOverlay();
      },
      feedback: setFeedback,
      restore(text) {
        setDraft((current) => current || text);
      },
      controls(online, retry) {
        setReady(online);
        setPending(retry);
      },
      connection(text, protocol) {
        setConnection(text);
        setProtocol(protocol);
      },
      stats(online, likes) {
        setStats("在线 " + online + " · 赞 " + likes);
      },
    });
    client.current = transport;
    return () => {
      transport.close();
      player?.();
      engine.close();
      client.current = null;
      overlay.current = null;
    };
  }, []);
  async function fullscreen() {
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else await stage.current?.requestFullscreen();
    } catch {
      setFeedback("浏览器未允许全屏");
    }
  }
  return (
    <>
      <header>
        <a href="/" className="brand">
          Maku
        </a>
        <span
          className="badge"
          id="connection"
          role="status"
          data-state={connection}
        >
          {connection}
        </span>
        <form id="join">
          {[
            ["room", "房间", room],
            ["session", "场次", session],
          ].map(([name, label, value]) => (
            <label key={name}>
              {label}
              <input
                name={name}
                defaultValue={value}
                required
                maxLength={24}
                pattern="[a-zA-Z0-9_\-]+"
              />
            </label>
          ))}
          <button>进入</button>
        </form>
      </header>
      <main>
        <section>
          <div className="screen" ref={stage}>
            <video hidden={!numeric} ref={video} muted controls playsInline />
            <canvas ref={canvas} aria-hidden="true" />
          </div>
          <div className="toolbar">
            <span>子协议 {protocol || "—"}</span>
            <Cursor client={client} />
            <span>{source}</span>
            <span>{stats}</span>
            <div className="tools">
              <Settings resetOverlay={() => overlay.current?.resetOverlay()} />
              <button onClick={fullscreen}>全屏</button>
            </div>
          </div>
          <form
            id="compose"
            onSubmit={(e) => {
              e.preventDefault();
              if (client.current?.send(draft)) setDraft("");
            }}
          >
            <input
              id="message"
              aria-label="弹幕内容"
              placeholder="写一条弹幕…"
              autoComplete="off"
              maxLength={500}
              required
              disabled={!ready}
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
            />
            <button id="send" className="primary" disabled={!ready || pending}>
              发送
            </button>
            <button
              type="button"
              disabled={!ready}
              onClick={() => client.current?.like()}
            >
              赞 +1
            </button>
          </form>
          <div className="feedback" role="status">
            {pending && (
              <button disabled={!ready} onClick={() => client.current?.retry()}>
                重试上一条
              </button>
            )}
            {feedback}
          </div>
        </section>
        <Feed client={client} feed={feed} />
      </main>
    </>
  );
}
