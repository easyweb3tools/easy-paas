import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  reactStrictMode: true,
  // typedRoutes is useful, but it makes simple shared components (like NavLink)
  // unnecessarily strict for this repo's current routing approach.
  trailingSlash: true,
  images: {
    unoptimized: true
  },
  async rewrites() {
    return [
      {
        source: "/api/market/:path*",
        destination: "http://127.0.0.1:8080/api/market/:path*",
      },
      {
        source: "/api/ai/:path*",
        destination: "http://127.0.0.1:8080/api/ai/:path*",
      },
    ];
  },
};

export default nextConfig;
