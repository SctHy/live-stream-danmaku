const protocolURL = "/assets/protocol.js";
const packetType: typeof import("/assets/protocol.js") = await import(
  /* @vite-ignore */ protocolURL
);

export type Message = {
  event_id: string;
  content: string;
  user_id: number;
  client_msg_id?: string;
};
type JournalEvent = { pos: string; prev: string } & (
  | { type: typeof packetType.danmaku; data: Message }
  | { type: typeof packetType.withdraw; data: { event_id: string } }
);
type Packet =
  | JournalEvent
  | {
      type: typeof packetType.stats;
      data: { online: number; likes: number; pos?: string };
    }
  | {
      type: typeof packetType.receipt;
      data: { client_msg_id?: string; status: string };
    }
  | { type: typeof packetType.ready | typeof packetType.migrate };
interface RoomView {
  show(message: Message, animate: boolean): void;
  withdraw(id: string): void;
  clear(): void;
  feedback(text: string): void;
  restore(text: string): void;
  controls(ready: boolean, pending: boolean): void;
  connection(text: string, protocol: string): void;
  stats(online: number, likes: number): void;
}
export function createRoomClient(room: string, session: string, ui: RoomView) {
  const base =
    "/api/rooms/" + [room, session].map(encodeURIComponent).join("/");
  const sockets = new Set<WebSocket>();
  let closed = false;
  let socket: WebSocket | undefined;
  let cursor = "",
    ready = false,
    retries = 0;
  let pendingSend: {
    started: number;
    data: { client_msg_id: string; content: string };
  } | null = null;
  let userID: number | null = null;
  const controls = () => ui.controls(ready, !!pendingSend);
  const feedback = ui.feedback;
  function apply(p: JournalEvent, animate: boolean) {
    if (p.type === packetType.withdraw) ui.withdraw(p.data.event_id);
    if (p.type === packetType.danmaku) {
      ui.show(p.data, animate);
      if (
        pendingSend &&
        p.data.user_id === userID &&
        p.data.client_msg_id === pendingSend.data.client_msg_id
      ) {
        pendingSend = null;
        controls();
        feedback("弹幕已送达");
      }
    }
    cursor = p.pos;
  }
  function older(a: string, b: string) {
    const [ae, aid] = a.includes("@") ? a.split("@") : ["", a];
    const [be, bid] = b.includes("@") ? b.split("@") : ["", b];
    if (ae !== be) return false;
    const [at, as] = aid.split("-").map(BigInt),
      [bt, bs] = bid.split("-").map(BigInt);
    return at < bt || (at === bt && as <= bs);
  }
  async function json<T>(url: string, options: RequestInit): Promise<T> {
    const r = await fetch(url, options);
    if (!r.ok) throw Error(String(r.status));
    return r.json();
  }
  let connecting = false;
  let retryTimer: ReturnType<typeof setTimeout> | undefined;
  async function connect() {
    if (closed || connecting) return;
    clearTimeout(retryTimer);
    connecting = true;
    try {
      const sessionInfo = await json<{ user_id: number }>("/api/session", {
        method: "POST",
        signal: AbortSignal.timeout(8000),
      });
      if (closed) return;
      if (userID !== sessionInfo.user_id && pendingSend) {
        pendingSend = null;
        feedback("身份已变更，上一条的结果请查看近期弹幕");
      }
      userID = sessionInfo.user_id;
      const ws = new WebSocket(
        location.origin.replace(/^http/, "ws") + base + "/live?migrate=1",
        "danmaku.batch.v1",
      );
      sockets.add(ws);
      let promoted = false;
      const deadline = setTimeout(() => ws.close(), 10000);
      let pending: JournalEvent[] = [],
        syncing = false;
      async function sync() {
        if (syncing) return;
        // A fresh Redis snapshot covers every event received before this request.
        pending = [];
        syncing = true;
        try {
          const after = cursor;
          const res = await json<{
            events: JournalEvent[];
            pos: string;
            reset: boolean;
          }>(base + "/recent?after=" + encodeURIComponent(after), {
            signal: AbortSignal.timeout(8000),
          });
          if (
            closed ||
            ws.readyState !== WebSocket.OPEN ||
            (promoted && socket !== ws)
          )
            return;
          // The old socket can advance the cursor while the snapshot is in flight.
          // Apply only the suffix beyond that cursor, including across a stream reset.
          let events = res.events;
          let reset = res.reset;
          let advance = true;
          if (cursor !== after) {
            const index = events.findIndex((p) => p.pos === cursor);
            if (index >= 0) {
              events = events.slice(index + 1);
              reset = false;
            } else if (cursor && older(res.pos, cursor)) {
              advance = false;
            } else {
              syncing = false;
              void sync();
              return;
            }
          }
          if (advance) {
            if (reset) {
              ui.clear();
              feedback("离线时间超出恢复窗口，已载入近期弹幕");
            }
            for (const p of events) apply(p, false);
            cursor = res.pos;
          }
          if (!promoted) {
            const previous = socket;
            socket = ws;
            promoted = true;
            connecting = false;
            clearTimeout(deadline);
            previous?.close();
          }
          syncing = false;
          const buffered = pending;
          pending = [];
          for (const p of buffered) ingest(p);
          if (syncing || ws.readyState !== WebSocket.OPEN) return;
          ready = true;
          retries = 0;
          controls();
          ui.connection("已连接", ws.protocol);
        } catch {
          ws.close();
        }
      }
      function ingest(p: JournalEvent) {
        if (syncing) {
          if (pending.length >= 512) ws.close();
          else pending.push(p);
          return;
        }
        if (cursor && older(p.pos, cursor)) return;
        if (p.prev !== (cursor || "0-0")) {
          pending.push(p);
          void sync();
          return;
        }
        apply(p, true);
      }
      ws.onmessage = ({ data }) => {
        if (closed || (promoted && ws !== socket)) return;
        const parsed: Packet | Packet[] = JSON.parse(data);
        for (const p of Array.isArray(parsed) ? parsed : [parsed]) {
          if (p.type === packetType.ready) {
            void sync();
          } else if (p.type === packetType.migrate) {
            if (promoted) void connect();
            else ws.close();
          } else if (p.type === packetType.stats && promoted) {
            ui.stats(p.data.online, p.data.likes);
            if (p.data.pos && cursor && !older(p.data.pos, cursor)) void sync();
          } else if (
            p.type === packetType.receipt &&
            (!p.data.client_msg_id ||
              p.data.client_msg_id === pendingSend?.data.client_msg_id)
          ) {
            if (
              pendingSend &&
              p.data.client_msg_id &&
              p.data.status !== "unavailable"
            ) {
              if (p.data.status !== "accepted")
                ui.restore(pendingSend.data.content);
              pendingSend = null;
            }
            controls();
            feedback(
              {
                accepted: "弹幕已发布",
                invalid: "内容无效",
                muted: "当前已禁言",
                rate_limited: "发送过快",
                filtered: "内容包含敏感词",
                overloaded: "网关繁忙，请稍后重试",
              }[p.data.status] || "服务暂不可用",
            );
          } else if (
            (p.type === packetType.danmaku || p.type === packetType.withdraw) &&
            p.pos
          )
            ingest(p);
        }
      };
      ws.onclose = () => {
        sockets.delete(ws);
        clearTimeout(deadline);
        if (promoted && socket !== ws) return;
        if (!promoted) connecting = false;
        reconnect();
      };
    } catch {
      connecting = false;
      reconnect();
    }
  }
  function reconnect() {
    if (closed) return;
    if (!wsOpen()) {
      ready = false;
      if (pendingSend) feedback("连接中断，最后一条弹幕的发送结果待确认");
      controls();
      ui.connection("正在重连", "");
    }
    if (closed || connecting) return;
    clearTimeout(retryTimer);
    const backoff = Math.min(8000, 300 * 2 ** retries++);
    retryTimer = setTimeout(connect, backoff / 2 + Math.random() * backoff);
  }
  function send(text: string) {
    if (pendingSend || !ready || !wsOpen() || !text.trim()) return false;
    pendingSend = {
      started: Date.now(),
      data: { client_msg_id: crypto.randomUUID(), content: text.trim() },
    };
    sendPending();
    feedback("发送中");
    return true;
  }
  function sendPending() {
    if (!pendingSend || !wsOpen() || !ready) return;
    if (Date.now() - pendingSend.started >= 9 * 60 * 1000) {
      pendingSend = null;
      feedback("重试期限已过，请在近期弹幕中确认");
    } else {
      socket?.send(
        JSON.stringify({ type: packetType.danmaku, data: pendingSend.data }),
      );
    }
    controls();
  }
  function wsOpen() {
    return socket?.readyState === WebSocket.OPEN;
  }
  function like() {
    if (wsOpen())
      socket?.send(
        JSON.stringify({ type: packetType.like, data: { count: 1 } }),
      );
  }
  async function moderate(
    action: "mute" | "withdraw",
    message: Message,
    token: string,
  ) {
    try {
      const response = await fetch(base + "/moderate", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer " + token,
        },
        body: JSON.stringify({
          action: action,
          event_id: message.event_id,
          user_id: message.user_id,
          seconds: 60,
        }),
      });
      feedback(
        response.ok
          ? "房管操作已执行"
          : response.status === 403
            ? "房管密钥或房间不匹配"
            : "房管操作失败",
      );
    } catch {
      feedback("房管操作失败");
    }
  }
  void connect();
  return {
    send,
    like,
    retry: sendPending,
    moderate,
    get cursor() {
      return cursor;
    },
    close() {
      closed = true;
      clearTimeout(retryTimer);
      for (const ws of sockets) ws.close();
    },
  };
}
