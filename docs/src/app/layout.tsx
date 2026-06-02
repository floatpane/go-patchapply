import type { Metadata } from "next";
import "./globals.css";
import "highlight.js/styles/github-dark.css";

export const metadata: Metadata = {
	title: "go-patchapply",
	description:
		"Apply parsed patches to files in Go. Add, modify, delete, rename; reverse and dry-run; path-confined and transactional. The apply half of go-mailpatch, and it never runs git.",
};

export default function RootLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<html lang="en">
			<body>
				<header className="site-header">
					<a href="/" className="brand">
						go-patchapply
					</a>
					<nav>
						<a href="https://github.com/floatpane/go-patchapply">GitHub</a>
					</nav>
				</header>
				<main>{children}</main>
			</body>
		</html>
	);
}

export const viewport = { width: "device-width", initialScale: 1 };
