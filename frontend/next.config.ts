import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Emit a self-contained server bundle so the Docker runtime stage needs no node_modules copy.
  // (Next 16 deployment guide → "Docker Standalone Output".)
  output: "standalone",
};

export default nextConfig;
