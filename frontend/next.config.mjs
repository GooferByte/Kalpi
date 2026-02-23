/** @type {import('next').NextConfig} */
const nextConfig = {
  // Required for the multi-stage Docker build
  output: 'standalone',

  // Proxy /api/* to the Go backend so the browser avoids CORS issues in dev
  async rewrites() {
    // API_URL is a server-side env var — not baked into the browser bundle.
    // In Docker: set API_URL=http://api:8080 so the Next.js server reaches the api container.
    // Locally: falls back to http://localhost:8080.
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.API_URL || 'http://localhost:8080'}/api/:path*`,
      },
    ]
  },
}

export default nextConfig
