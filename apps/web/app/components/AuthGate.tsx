"use client";

import { useEffect, useState, ReactNode } from "react";
import { useRouter } from "next/navigation";

type User = {
    id: string;
    email: string;
    username: string;
    display_name: string;
    avatar_url?: string | null;
};

export default function AuthGate({
    children,
}: {
    children: ReactNode;
}) {
    const router = useRouter();
    const [user, setUser] = useState<User | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        async function checkAuth() {
            try {
                const response = await fetch("/api/auth/me", {
                    cache: "no-store",
                });

                if (!response.ok) {
                    router.replace("/login");
                    return;
                }

                const data = await response.json();
                setUser(data.user);
            } catch {
                router.replace("/login");
            } finally {
                setLoading(false);
            }
        }

        checkAuth();
    }, [router]);

    async function logout() {
        try {
            await fetch("/api/auth/logout", {
                method: "POST",
            });

            router.replace("/login");
            router.refresh();
        } catch {
            alert("Unable to logout. Please try again.");
        }
    }

    if (loading || !user) {
        return (
            <div className="auth-loading">
                Loading Relay...
            </div>
        );
    }

    return (
        <>
            <div className="auth-user-bar">
                <span>
                    Signed in as <strong>{user.display_name}</strong>
                </span>

                <button onClick={logout}>Logout</button>
            </div>

            {children}
        </>
    );
}