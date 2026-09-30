import type { Metadata } from "next";
import { Geist, Geist_Mono, Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";
import { Nav } from "@/shared/layout/Nav";
import { NavSlot } from "@/shared/layout/NavSlot";
import { getTheme } from "@/shared/lib/theme";
import { Banner } from "@/shared/layout/Banner";
import { ToastContainer } from "@/shared/layout/ToastContainer";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });
const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
  axes: ["opsz"],
  display: "swap",
});
const jetbrainsMono = JetBrains_Mono({
  variable: "--font-jetbrains-mono",
  subsets: ["latin"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "Codritium",
  description:
    "Practice real engineering problems alongside your AI tool. Five-dimension scoring, anti-pattern detection, daily streak.",
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const theme = await getTheme();
  return (
    <html
      lang="en"
      data-theme={theme}
      className={`${geistSans.variable} ${geistMono.variable} ${inter.variable} ${jetbrainsMono.variable}`}
    >
      <body className="antialiased min-h-screen" suppressHydrationWarning>
        <Banner />
        <NavSlot>
          <Nav />
        </NavSlot>
        <main>{children}</main>
        <ToastContainer />
      </body>
    </html>
  );
}
