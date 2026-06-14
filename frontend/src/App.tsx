import { FormEvent, KeyboardEvent, RefObject, useEffect, useMemo, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { AppService } from "../bindings/echo";

type Mode = "launcher" | "chat";
type Role = "user" | "assistant";

type Message = {
  id: string;
  role: Role;
  content: string;
  actions?: AssistantAction[];
};

type AssistantAction = {
  kind: string;
  status: string;
  title: string;
  detail: string;
  eventId: string;
  link: string;
  time: string;
};

type CalendarStatus = {
  connected: boolean;
  message: string;
  account: string;
  error: string;
};

const starterPrompts = [
  "What is on my calendar tomorrow?",
  "Schedule focus time Friday at 10am",
  "Do I have anything this afternoon?",
];

function App() {
  const [mode, setMode] = useState<Mode>("launcher");
  const [input, setInput] = useState("");
  const [messages, setMessages] = useState<Message[]>([]);
  const [actions, setActions] = useState<AssistantAction[]>([]);
  const [isThinking, setIsThinking] = useState(false);
  const [calendar, setCalendar] = useState<CalendarStatus>({
    connected: false,
    message: "Checking Google Calendar...",
    account: "",
    error: "",
  });

  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
  const threadRef = useRef<HTMLDivElement | null>(null);

  const latestAssistantText = useMemo(() => {
    const assistantMessages = messages.filter((message) => message.role === "assistant");
    return assistantMessages[assistantMessages.length - 1]?.content ?? "Ask me to read or write Google Calendar.";
  }, [messages]);

  useEffect(() => {
    refreshCalendarStatus();
  }, []);

  useEffect(() => {
    const offLauncher = Events.On("echo:launcher", () => {
      setMode("launcher");
      setInput("");
      requestAnimationFrame(() => inputRef.current?.focus());
    });

    const onFocus = () => {
      requestAnimationFrame(() => inputRef.current?.focus());
    };

    window.addEventListener("focus", onFocus);
    return () => {
      offLauncher();
      window.removeEventListener("focus", onFocus);
    };
  }, []);

  useEffect(() => {
    threadRef.current?.scrollTo({
      top: threadRef.current.scrollHeight,
      behavior: "smooth",
    });
  }, [messages, isThinking]);

  async function refreshCalendarStatus() {
    try {
      const status = await AppService.CalendarStatus();
      setCalendar(status as CalendarStatus);
    } catch (error) {
      setCalendar({
        connected: false,
        message: "Google Calendar status unavailable.",
        account: "",
        error: String(error),
      });
    }
  }

  async function connectCalendar() {
    setCalendar((current) => ({
      ...current,
      message: "Waiting for Google authorization...",
      error: "",
    }));

    const status = await AppService.ConnectGoogleCalendar();
    setCalendar(status as CalendarStatus);
  }

  async function submitPrompt(event?: FormEvent) {
    event?.preventDefault();
    const prompt = input.trim();
    if (!prompt || isThinking) {
      return;
    }

    const userMessage: Message = {
      id: crypto.randomUUID(),
      role: "user",
      content: prompt,
    };
    const nextMessages = [...messages, userMessage];

    setInput("");
    setMessages(nextMessages);
    setIsThinking(true);

    if (mode === "launcher") {
      setMode("chat");
      await AppService.SetWindowMode("chat");
    }

    try {
      const response = await AppService.Chat({
        prompt,
        messages: nextMessages.map((message) => ({
          id: message.id,
          role: message.role,
          content: message.content,
        })),
      });

      const responseActions = ((response.actions ?? []) as AssistantAction[]).filter(Boolean);
      if (responseActions.length > 0) {
        setActions((current) => [...responseActions, ...current].slice(0, 12));
        refreshCalendarStatus();
      }

      setMessages((current) => [
        ...current,
        {
          id: crypto.randomUUID(),
          role: "assistant",
          content: response.message || "Done.",
          actions: responseActions,
        },
      ]);
    } catch (error) {
      setMessages((current) => [
        ...current,
        {
          id: crypto.randomUUID(),
          role: "assistant",
          content: "I hit an app error while handling that request.",
          actions: [
            {
              kind: "app",
              status: "failed",
              title: "Assistant request failed",
              detail: String(error),
              eventId: "",
              link: "",
              time: new Date().toLocaleTimeString([], { hour: "numeric", minute: "2-digit" }),
            },
          ],
        },
      ]);
    } finally {
      setIsThinking(false);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }

  function onComposerKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      submitPrompt();
    }
  }

  function openLauncher() {
    setMode("launcher");
    AppService.SetWindowMode("launcher");
    requestAnimationFrame(() => inputRef.current?.focus());
  }

  if (mode === "launcher") {
    return (
      <main className="launcher-shell" aria-label="Echo launcher">
        <form className="launcher-bar" onSubmit={submitPrompt}>
          <div className="brand-mark" aria-hidden="true">
            E
          </div>
          <label className="launcher-label" htmlFor="launcher-input">
            <span>Echo</span>
            <small>{calendar.connected ? "Calendar connected" : "Calendar not connected"}</small>
          </label>
          <input
            id="launcher-input"
            ref={inputRef as RefObject<HTMLInputElement>}
            value={input}
            onChange={(event) => setInput(event.target.value)}
            placeholder="Ask Echo or schedule with Google Calendar..."
            autoFocus
          />
          <button className="send-button" type="submit" disabled={!input.trim() || isThinking}>
            Enter
          </button>
        </form>
      </main>
    );
  }

  return (
    <main className="app-shell" aria-label="Echo chat">
      <header className="topbar">
        <button className="mark-button" type="button" onClick={openLauncher} aria-label="Return to launcher">
          E
        </button>
        <div className="topbar-copy">
          <h1>Echo</h1>
          <p>{latestAssistantText}</p>
        </div>
        <div className="topbar-actions">
          <button
            className={`calendar-button ${calendar.connected ? "is-connected" : ""}`}
            type="button"
            onClick={calendar.connected ? refreshCalendarStatus : connectCalendar}
          >
            {calendar.connected ? "Calendar ready" : "Connect Calendar"}
          </button>
          <button className="ghost-button" type="button" onClick={() => AppService.HideWindow()}>
            Hide
          </button>
        </div>
      </header>

      <section className="workspace">
        <div className="thread-panel">
          <div className="thread" ref={threadRef} aria-live="polite">
            {messages.length === 0 ? (
              <div className="empty-state">
                <p>Start with Google Calendar.</p>
                <div className="prompt-row">
                  {starterPrompts.map((prompt) => (
                    <button key={prompt} type="button" onClick={() => setInput(prompt)}>
                      {prompt}
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              messages.map((message) => (
                <article className={`message ${message.role}`} key={message.id}>
                  <div className="message-meta">{message.role === "user" ? "You" : "Echo"}</div>
                  <div className="message-bubble">{message.content}</div>
                  {message.actions && message.actions.length > 0 ? (
                    <div className="inline-actions">
                      {message.actions.map((action, index) => (
                        <ActionPill action={action} key={`${action.title}-${index}`} />
                      ))}
                    </div>
                  ) : null}
                </article>
              ))
            )}
            {isThinking ? (
              <article className="message assistant">
                <div className="message-meta">Echo</div>
                <div className="message-bubble thinking">
                  <span />
                  <span />
                  <span />
                </div>
              </article>
            ) : null}
          </div>

          <form className="composer" onSubmit={submitPrompt}>
            <textarea
              ref={inputRef as RefObject<HTMLTextAreaElement>}
              value={input}
              onChange={(event) => setInput(event.target.value)}
              onKeyDown={onComposerKeyDown}
              placeholder="Message Echo..."
              rows={2}
            />
            <button className="send-button" type="submit" disabled={!input.trim() || isThinking}>
              Send
            </button>
          </form>
        </div>

        <aside className="activity-panel" aria-label="Google Calendar activity">
          <div className="panel-heading">
            <span>Calendar</span>
            <strong className={calendar.connected ? "status-ok" : "status-warn"}>
              {calendar.connected ? "Connected" : "Setup"}
            </strong>
          </div>
          <p className="calendar-status">{calendar.message}</p>

          <div className="activity-list">
            {actions.length === 0 ? (
              <div className="quiet-note">Calendar reads and writes will appear here.</div>
            ) : (
              actions.map((action, index) => <ActionCard action={action} key={`${action.title}-${index}`} />)
            )}
          </div>
        </aside>
      </section>
    </main>
  );
}

function ActionPill({ action }: { action: AssistantAction }) {
  return (
    <div className={`action-pill ${action.status}`}>
      <span>{action.title}</span>
      <small>{action.status}</small>
    </div>
  );
}

function ActionCard({ action }: { action: AssistantAction }) {
  return (
    <article className={`action-card ${action.status}`}>
      <div>
        <span className="action-kind">{action.kind}</span>
        <time>{action.time}</time>
      </div>
      <h2>{action.title}</h2>
      <p>{action.detail}</p>
      {action.link ? (
        <button type="button" onClick={() => window.open(action.link)}>
          Open event
        </button>
      ) : null}
    </article>
  );
}

export default App;
