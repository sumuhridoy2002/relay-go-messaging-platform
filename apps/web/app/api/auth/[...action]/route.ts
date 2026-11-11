
import { NextRequest, NextResponse } from "next/server";

const API_URL =
    process.env.RELAY_API_URL || "http://localhost:8080";

const COOKIE_NAME = "relay_token";

// LOGIN, REGISTER AND LOGOUT
export async function POST(request: NextRequest) {
    const action = request.nextUrl.pathname.split("/").pop();

    // LOGOUT
    if (action === "logout") {
        const response = NextResponse.json({
            message: "Logged out successfully",
        });

        response.cookies.delete(COOKIE_NAME);
        return response;
    }

    // Only login and register are allowed below
    if (action !== "login" && action !== "register") {
        return NextResponse.json(
            { message: "Invalid action" },
            { status: 404 }
        );
    }

    try {
        const body = await request.json();

        const apiResponse = await fetch(
            `${API_URL}/api/v1/auth/${action}`,
            {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(body),
                cache: "no-store",
            }
        );

        const data = await apiResponse.json();

        if (!apiResponse.ok) {
            return NextResponse.json(data, {
                status: apiResponse.status,
            });
        }

        const response = NextResponse.json({
            user: data.user,
            message: "Authentication successful",
        });

        if (data.access_token) {
            response.cookies.set(COOKIE_NAME, data.access_token, {
                httpOnly: true,
                secure: process.env.NODE_ENV === "production",
                sameSite: "lax",
                path: "/",
                maxAge: 60 * 60 * 24,
            });
        }

        return response;

    } catch (error) {
        console.error("Auth API error:", error);

        return NextResponse.json(
            { message: "Unable to connect to the API" },
            { status: 502 }
        );
    }
}

// GET LOGGED-IN USER PROFILE
export async function GET(request: NextRequest) {
    const action = request.nextUrl.pathname.split("/").pop();

    if (action !== "me") {
        return NextResponse.json(
            { message: "Invalid action" },
            { status: 404 }
        );
    }

    const token = request.cookies.get(COOKIE_NAME)?.value;

    if (!token) {
        return NextResponse.json(
            { message: "Unauthenticated" },
            { status: 401 }
        );
    }

    try {
        const apiResponse = await fetch(
            `${API_URL}/api/v1/me`,
            {
                headers: {
                    Authorization: `Bearer ${token}`,
                },
                cache: "no-store",
            }
        );

        const data = await apiResponse.json();

        const response = NextResponse.json(data, {
            status: apiResponse.status,
        });

        if (apiResponse.status === 401) {
            response.cookies.delete(COOKIE_NAME);
        }

        return response;

    } catch (error) {
        console.error("Profile API error:", error);

        return NextResponse.json(
            { message: "Unable to connect to the API" },
            { status: 502 }
        );
    }
}