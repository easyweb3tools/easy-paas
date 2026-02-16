import type { NextConfig } from "next";

// The web UI should only talk to easyweb3-platform (PaaS gateway).
// easyweb3-platform then proxies to polymarket-backend via /api/v1/services/polymarket/*.
const PLATFORM_INTERNAL = process.env.EASYWEB3_PLATFORM_INTERNAL_URL ?? "http://easyweb3-platform:8080";

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
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          {
            key: "Content-Security-Policy",
            value: [
              "default-src 'self'",
              "base-uri 'self'",
              "frame-ancestors 'none'",
              "object-src 'none'",
              "img-src 'self' data: blob:",
              "font-src 'self' data:",
              "style-src 'self' 'unsafe-inline'",
              "script-src 'self' 'unsafe-inline'",
              "connect-src 'self' https:",
              "upgrade-insecure-requests",
            ].join("; "),
          },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
        ],
      },
    ];
  },
  async rewrites() {
    return [
      // Browser fetches /api/v2/* to same-origin. Next server proxies to PaaS gateway inside docker network.
      {
        source: "/api/v2/:path*",
        destination: `${PLATFORM_INTERNAL}/api/v1/services/polymarket/api/v2/:path*`,
      },
      // Allow UI to call platform APIs (auth/logs/etc) using same-origin /api/v1/*.
      {
        source: "/api/v1/:path*",
        destination: `${PLATFORM_INTERNAL}/api/v1/:path*`,
      },
      {
        source: "/healthz",
        destination: `${PLATFORM_INTERNAL}/healthz`,
      },
    ];
  },
};

export default nextConfig;
