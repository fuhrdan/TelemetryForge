import "./globals.css";

export const metadata = {
  title: "TelemetryForge",
  description: "Real-time telemetry control plane dashboard",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
