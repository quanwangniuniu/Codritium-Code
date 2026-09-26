import type { NextConfig } from "next";

const API_URL = process.env.CODRITIUM_API_URL ?? "http://localhost:8080";

// All /api/* requests are proxied to the Go backend so the browser always sees
// same-origin cookies. In production this rewrite stays the same; only the env
// var changes.
const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${API_URL}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
