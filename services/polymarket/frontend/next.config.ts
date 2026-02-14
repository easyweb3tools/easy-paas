import type { NextConfig } from "next";

const BACKEND_INTERNAL = process.env.POLYMARKET_BACKEND_INTERNAL_URL ?? "http://polymarket-backend-web:8080";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // typedRoutes is useful, but it makes simple shared components (like NavLink)
  // unnecessarily strict for this repo's current routing approach.
  // Run as a Next.js server in docker-compose. (Static export doesn't work with dynamic routes.)
  output: "standalone",
  trailingSlash: true,
  images: {
    unoptimized: true
  },
  async rewrites() {
    return [
      // Browser fetches /api/v2/* to same-origin. Next server proxies to backend inside docker network.
      {
        source: "/api/v2/:path*",
        destination: `${BACKEND_INTERNAL}/api/v2/:path*`,
      },
      {
        source: "/healthz",
        destination: `${BACKEND_INTERNAL}/healthz`,
      },
    ];
  },
};

export default nextConfig;
