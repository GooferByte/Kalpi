/** @type {import('next').NextConfig} */
const nextConfig = {
  // Required for the multi-stage Docker build
  output: 'standalone',

  // Proxy /api/* to the Go backend so the browser avoids CORS issues in dev
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/:path*`,
      },
    ]
  },
}

export default nextConfig
