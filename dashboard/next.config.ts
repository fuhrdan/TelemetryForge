import type { NextConfig } from "next";

const apiBase = process.env.TELEMETRYFORGE_API_BASE ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/telemetry-api/:path*",
        destination: `${apiBase}/:path*`,
      },
    ];
  },
};

export default nextConfig;
