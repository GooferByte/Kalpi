import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
  title: 'Kalpi — Portfolio Execution Engine',
  description: 'Execute systematic portfolio trades across Indian brokers in one click',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  )
}
