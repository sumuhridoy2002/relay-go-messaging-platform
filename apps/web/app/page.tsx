
"use client";

import { useCallback, useEffect, useState } from "react";
import AuthGate from "./components/AuthGate";
import NewMessageModal from "./components/NewMessageModal";

type Conversation = {
    id: string;
    kind: "direct" | "group";
    title: string | null;
    created_at: string;
    last_message: string | null;
    last_message_type: string | null;
    last_message_at: string | null;
    member_count: number;
    other_member_name: string | null;
    other_member_username: string | null;
    other_member_avatar: string | null;
};

type Message = {
    id: string;
    conversation_id: string;
    sender_id: string;
    display_name: string;
    username: string;
    avatar_url: string | null;
    body: string;
    message_type: string;
    created_at: string;
    edited_at: string | null;
};

function formatTime(date: string | null) {
    if (!date) return "";

    return new Date(date).toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
    });
}

function getConversationName(chat: Conversation) {
    return (
        chat.title ||
        chat.other_member_name ||
        (chat.kind === "direct" ? "Direct message" : "Group")
    );
}

function ConversationAvatar({
    conversation,
    className,
}: {
    conversation: Conversation;
    className: string;
}) {
    const name = getConversationName(conversation);

    return (
        <span className={className}>
            {conversation.other_member_avatar ? (
                <img
                    src={conversation.other_member_avatar}
                    alt={name}
                    className="avatar-image"
                />
            ) : conversation.kind === "group" ? (
                "👥"
            ) : (
                name[0]?.toUpperCase() || "U"
            )}
        </span>
    );
}

export default function Home() {
    const [conversations, setConversations] = useState<Conversation[]>([]);
    const [messages, setMessages] = useState<Message[]>([]);
    const [selectedId, setSelectedId] = useState("");
    const [userId, setUserId] = useState("");
    const [displayName, setDisplayName] = useState("You");
    const [message, setMessage] = useState("");
    const [loading, setLoading] = useState(true);
    const [sending, setSending] = useState(false);
    const [error, setError] = useState("");
    const [showNewMessage, setShowNewMessage] = useState(false);

    const selectedConversation = conversations.find(
        (conversation) => conversation.id === selectedId
    );

    const loadConversations = useCallback(async () => {
        try {
            const response = await fetch("/api/chat/conversations", {
                credentials: "include",
                cache: "no-store",
            });

            if (!response.ok) {
                throw new Error("Failed to load conversations");
            }

            const data = await response.json();
            const items: Conversation[] = data.conversations ?? [];

            setConversations(items);

            setSelectedId((current) => {
                if (current && items.some((item) => item.id === current)) {
                    return current;
                }

                return items[0]?.id ?? "";
            });
        } catch (err) {
            setError(
                err instanceof Error ? err.message : "Something went wrong"
            );
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        loadConversations();

        fetch("/api/auth/me")
            .then((response) => response.json())
            .then((data) => {
                const user = data.user ?? data.data?.user ?? data;

                if (user.id) setUserId(user.id);
                if (user.display_name) setDisplayName(user.display_name);
            })
            .catch(console.error);
    }, [loadConversations]);

    useEffect(() => {
        if (!selectedId) {
            setMessages([]);
            return;
        }

        let cancelled = false;

        async function loadMessages() {
            try {
                setError("");

                const response = await fetch(
                    `/api/chat/conversations/${selectedId}/messages`,
                    {
                        credentials: "include",
                        cache: "no-store",
                    }
                );

                if (!response.ok) {
                    throw new Error("Failed to load messages");
                }

                const data = await response.json();

                if (!cancelled) {
                    setMessages(data.messages ?? []);
                }
            } catch (err) {
                if (!cancelled) {
                    setError(
                        err instanceof Error
                            ? err.message
                            : "Unable to load messages"
                    );
                }
            }
        }

        loadMessages();

        return () => {
            cancelled = true;
        };
    }, [selectedId]);

    async function sendMessage(event: React.FormEvent<HTMLFormElement>) {
        event.preventDefault();

        const body = message.trim();

        if (!body || !selectedId || sending) return;

        setSending(true);
        setError("");

        try {
            const response = await fetch(
                `/api/chat/conversations/${selectedId}/messages`,
                {
                    method: "POST",
                    credentials: "include",
                    headers: {
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify({
                        body,
                        message_type: "text",
                    }),
                }
            );

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || "Message could not be sent");
            }

            const sentMessage: Message = {
                ...data.data,
                display_name: displayName,
                username: "",
                avatar_url: null,
                edited_at: null,
            };

            setMessages((current) => [...current, sentMessage]);
            setMessage("");

            setConversations((current) =>
                current.map((conversation) =>
                    conversation.id === selectedId
                        ? {
                              ...conversation,
                              last_message: body,
                              last_message_type: "text",
                              last_message_at: sentMessage.created_at,
                          }
                        : conversation
                )
            );
        } catch (err) {
            setError(
                err instanceof Error ? err.message : "Unable to send message"
            );
        } finally {
            setSending(false);
        }
    }

    async function openConversation(conversationId: string) {
        setShowNewMessage(false);

        await loadConversations();

        setSelectedId(conversationId);
        setMessages([]);
        setError("");
    }

    return (
        <AuthGate>
            <main className="shell">
                <aside className="sidebar">
                    <div className="brand">
                        <span className="brand-mark">R</span>
                        <span>relay</span>
                    </div>

                    <div className="profile">
                        <div className="avatar me">
                            {displayName[0]?.toUpperCase()}
                        </div>

                        <div>
                            <b>{displayName}</b>
                            <small>Available</small>
                        </div>

                        <span className="online-dot" />
                    </div>

                    <div className="section-label">
                        MESSAGES

                        <button
                            aria-label="New message"
                            title="New message"
                            onClick={() => setShowNewMessage(true)}
                        >
                            ＋
                        </button>
                    </div>

                    <label className="search">
                        <span>⌕</span>
                        <input placeholder="Search conversations" />
                    </label>

                    <nav className="chat-list">
                        {loading && <p>Loading conversations...</p>}

                        {!loading && conversations.length === 0 && (
                            <p>No conversations yet.</p>
                        )}

                        {conversations.map((chat, index) => {
                            const name = getConversationName(chat);

                            return (
                                <button
                                    className={`chat ${
                                        selectedId === chat.id ? "selected" : ""
                                    }`}
                                    key={chat.id}
                                    onClick={() => setSelectedId(chat.id)}
                                >
                                    <ConversationAvatar
                                        conversation={chat}
                                        className={`avatar avatar-${index % 4}`}
                                    />

                                    <span className="chat-copy">
                                        <b>{name}</b>

                                        <small>
                                            {chat.last_message ||
                                                "No messages yet"}
                                        </small>
                                    </span>

                                    <span className="chat-meta">
                                        <small>
                                            {formatTime(chat.last_message_at)}
                                        </small>
                                    </span>
                                </button>
                            );
                        })}
                    </nav>

                    <div className="sidebar-bottom">
                        <button>⚙ Settings</button>

                        <button
                            onClick={() => setShowNewMessage(true)}
                        >
                            ＋ New group
                        </button>
                    </div>
                </aside>

                <section className="conversation">
                    <header className="topbar">
                        {selectedConversation ? (
                            <>
                                <div className="room">
                                    <ConversationAvatar
                                        conversation={selectedConversation}
                                        className="avatar avatar-0"
                                    />

                                    <div>
                                        <b>
                                            {getConversationName(
                                                selectedConversation
                                            )}
                                        </b>

                                        <small>
                                            {selectedConversation.member_count}{" "}
                                            members
                                        </small>
                                    </div>
                                </div>

                                <div className="actions">
                                    <button title="Audio call">☎</button>
                                    <button title="Video call">▣</button>
                                    <button title="More options">···</button>
                                </div>
                            </>
                        ) : (
                            <div className="room">
                                <b>Select a conversation</b>
                            </div>
                        )}
                    </header>

                    <div className="messages">
                        {error && (
                            <p className="error-message">{error}</p>
                        )}

                        {messages.length > 0 && (
                            <div className="day-divider">
                                <span>MESSAGES</span>
                            </div>
                        )}

                        {messages.map((msg) => {
                            const own = msg.sender_id === userId;

                            return (
                                <div
                                    className={`message ${own ? "own" : ""}`}
                                    key={msg.id}
                                >
                                    {!own && (
                                        <span className="avatar avatar-1">
                                            {msg.avatar_url ? (
                                                <img
                                                    src={msg.avatar_url}
                                                    alt={msg.display_name}
                                                    className="avatar-image"
                                                />
                                            ) : (
                                                msg.display_name?.[0]?.toUpperCase() ||
                                                "U"
                                            )}
                                        </span>
                                    )}

                                    <div>
                                        <b>
                                            {own ? "You" : msg.display_name}
                                            <small>
                                                {" "}
                                                {formatTime(msg.created_at)}
                                            </small>
                                        </b>

                                        <p>{msg.body}</p>
                                    </div>
                                </div>
                            );
                        })}

                        {selectedId && messages.length === 0 && (
                            <p>No messages yet. Start a conversation!</p>
                        )}

                        {!selectedId && !loading && (
                            <p>Select a conversation or start a new message.</p>
                        )}
                    </div>

                    <form className="composer" onSubmit={sendMessage}>
                        <button type="button" aria-label="Attach file">
                            ＋
                        </button>

                        <input
                            placeholder="Write a message..."
                            value={message}
                            onChange={(event) =>
                                setMessage(event.target.value)
                            }
                            disabled={!selectedId || sending}
                        />

                        <button type="button" aria-label="Emoji">
                            ☺
                        </button>

                        <button
                            className="send"
                            type="submit"
                            disabled={
                                !selectedId || !message.trim() || sending
                            }
                        >
                            {sending ? "..." : "➤"}
                        </button>
                    </form>
                </section>
            </main>

            {showNewMessage && (
                <NewMessageModal
                    onClose={() => setShowNewMessage(false)}
                    onOpenConversation={openConversation}
                />
            )}
        </AuthGate>
    );
}
