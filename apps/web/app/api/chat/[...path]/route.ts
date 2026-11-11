
import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";

const API_URL = process.env.RELAY_API_URL || "http://localhost:8080";

async function proxy(request: NextRequest) {
    try {
        const cookieStore = await cookies();
        const token = cookieStore.get("relay_token")?.value;

        if (!token) {
            return NextResponse.json(
                { error: "Unauthorized" },
                { status: 401 }
            );
        }

        const path = request.nextUrl.pathname
            .replace(/^\/api\/chat\/?/, "")
            .split("/")
            .map(encodeURIComponent)
            .join("/");

        const query = request.nextUrl.search;

        const headers = new Headers({
            Authorization: `Bearer ${token}`,
        });

        const contentType = request.headers.get("content-type");
        if (contentType) {
            headers.set("Content-Type", contentType);
        }

        const options: RequestInit = {
            method: request.method,
            headers,
            cache: "no-store",
        };

        if (!["GET", "HEAD"].includes(request.method)) {
            options.body = await request.text();
        }

        const response = await fetch(
            `${API_URL}/api/v1/${path}${query}`,
            options
        );

        const data = await response.text();

        return new NextResponse(data, {
            status: response.status,
            headers: {
                "Content-Type":
                    response.headers.get("content-type") ||
                    "application/json",
            },
        });
    } catch {
        return NextResponse.json(
            { error: "Unable to connect to Relay API" },
            { status: 502 }
        );
    }
}

export const GET = proxy;
export const POST = proxy;
