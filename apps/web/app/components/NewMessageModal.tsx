
"use client";

import { useEffect, useState } from "react";

type User = {
    id: string;
    username: string;
    display_name: string;
    avatar_url: string | null;
};

type Conversation = {
    id: string;
    kind: "direct" | "group";
    other_member_username: string | null;
};

type Props = {
    onClose: () => void;
    onOpenConversation: (conversationId: string) => void;
};

export default function NewMessageModal({
    onClose,
    onOpenConversation,
}: Props) {
    const [query, setQuery] = useState("");
    const [users, setUsers] = useState<User[]>([]);
    const [loading, setLoading] = useState(false);
    const [creating, setCreating] = useState<string | null>(null);
    const [error, setError] = useState("");

    useEffect(() => {
        const search = query.trim();

        if (search.length < 2) {
            setUsers([]);
            setError("");
            return;
        }

        const controller = new AbortController();

        const timer = setTimeout(async () => {
            try {
                setLoading(true);
                setError("");

                const response = await fetch(
                    `/api/chat/users/search?q=${encodeURIComponent(search)}`,
                    {
                        method: "GET",
                        credentials: "include",
                        signal: controller.signal,
                    }
                );

                const data = await response.json();

                if (!response.ok) {
                    throw new Error(data.error || "User search failed");
                }

                setUsers(data.users ?? []);
            } catch (err) {
                if (err instanceof Error && err.name !== "AbortError") {
                    setError(err.message);
                }
            } finally {
                if (!controller.signal.aborted) {
                    setLoading(false);
                }
            }
        }, 300);

        return () => {
            clearTimeout(timer);
            controller.abort();
        };
    }, [query]);

    async function handleSelect(user: User) {
        if (creating) return;

        try {
            setCreating(user.id);
            setError("");

            // Load existing conversations.
            const conversationsResponse = await fetch(
                "/api/chat/conversations",
                {
                    credentials: "include",
                    cache: "no-store",
                }
            );

            const conversationsData = await conversationsResponse.json();

            if (!conversationsResponse.ok) {
                throw new Error(
                    conversationsData.error || "Could not load conversations"
                );
            }

            const conversations: Conversation[] =
                conversationsData.conversations ?? [];

            // Reuse existing direct conversation.
            const existing = conversations.find(
                (conversation) =>
                    conversation.kind === "direct" &&
                    conversation.other_member_username === user.username
            );

            if (existing) {
                onOpenConversation(existing.id);
                return;
            }

            // Create new direct conversation.
            const response = await fetch("/api/chat/conversations", {
                method: "POST",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    kind: "direct",
                    member_ids: [user.id],
                }),
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(
                    data.error || "Could not create conversation"
                );
            }

            const conversationId =
                data.conversation?.id ||
                data.data?.id ||
                data.id;

            if (!conversationId) {
                throw new Error(
                    "Conversation ID was not returned by the API"
                );
            }

            onOpenConversation(conversationId);
        } catch (err) {
            setError(
                err instanceof Error
                    ? err.message
                    : "Something went wrong"
            );
        } finally {
            setCreating(null);
        }
    }

    return (
        <div className="new-message-overlay" onClick={onClose}>
            <div
                className="new-message-modal"
                onClick={(event) => event.stopPropagation()}
            >
                <div className="new-message-header">
                    <h2>New Message</h2>

                    <button
                        type="button"
                        onClick={onClose}
                        aria-label="Close"
                    >
                        ×
                    </button>
                </div>

                <input
                    autoFocus
                    type="text"
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    placeholder="Search by username or name..."
                    className="new-message-search"
                />

                <div className="new-message-results">
                    {query.trim().length < 2 ? (
                        <p className="new-message-hint">
                            Enter at least 2 characters to search.
                        </p>
                    ) : loading ? (
                        <p className="new-message-hint">
                            Searching users...
                        </p>
                    ) : error ? (
                        <p className="new-message-error">{error}</p>
                    ) : users.length === 0 ? (
                        <p className="new-message-hint">
                            No users found.
                        </p>
                    ) : (
                        users.map((user) => (
                            <button
                                type="button"
                                key={user.id}
                                className="new-message-user"
                                onClick={() => handleSelect(user)}
                                disabled={creating !== null}
                            >
                                {user.avatar_url ? (
                                    <img
                                        src={user.avatar_url}
                                        alt={user.display_name}
                                        className="new-message-avatar"
                                    />
                                ) : (
                                    <span className="new-message-avatar new-message-avatar-fallback">
                                        {user.display_name[0]?.toUpperCase() ||
                                            "U"}
                                    </span>
                                )}

                                <span className="new-message-user-info">
                                    <b>{user.display_name}</b>
                                    <small>@{user.username}</small>
                                </span>

                                {creating === user.id && (
                                    <span className="new-message-opening">
                                        Opening...
                                    </span>
                                )}
                            </button>
                        ))
                    )}
                </div>
            </div>
        </div>
    );
}
