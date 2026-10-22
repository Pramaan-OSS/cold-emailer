import { defineConfig, loadEnv, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";
import tailwindcss from "@tailwindcss/vite";

import { dashboardCspPlugin, noncePlaceholder } from "./csp";

// Requests the interface font from the head, in parallel with the scripts, not after the stylesheet parses.
function preloadInterfaceFont(): Plugin {
    return {
        name: "coldemail:preload-interface-font",
        apply: "build",
        transformIndexHtml: {
            order: "post",
            handler(_html, ctx) {
                const file = Object.keys(ctx.bundle ?? {}).find((name) => /inter-latin-opsz-normal-[^/]*\.woff2$/.test(name));
                if (!file) return [];
                return [
                    {
                        tag: "link",
                        attrs: { rel: "preload", as: "font", type: "font/woff2", href: `/${file}`, crossorigin: "" },
                        injectTo: "head-prepend",
                    },
                ];
            },
        },
    };
}

export default defineConfig(({ command, mode, isPreview }) => ({
    html: { cspNonce: command === "serve" && !isPreview ? noncePlaceholder : undefined },
    plugins: [
        react(),
        tailwindcss(),
        preloadInterfaceFont(),

        dashboardCspPlugin({ ...loadEnv(mode, process.cwd(), ""), ...process.env } as Record<string, string>),
    ],
    build: {
        // Only when they are going to be uploaded: shipping them otherwise
        // would hand every visitor the app's original sources.
        sourcemap: false,
    },
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
    // Pre-bundle heavy deps so the first request to the dev server
    // doesn't trigger a cold compile of axios/framer-motion/etc. This
    // shaves ~300-500ms off the first paint locally.
    optimizeDeps: {
        include: [
            "react",
            "react-dom",
            "@tanstack/react-router",
            "axios",
            "framer-motion",
            "@tanstack/react-query",
            "@tanstack/react-query-devtools",
            "react-hot-toast/headless",
            "lucide-react",
            "@remixicon/react",
        ],
    },
    server: {
        // Permit Tailscale MagicDNS names (and any extra hosts via
        // VITE_ALLOWED_HOSTS) when the server is exposed with --host. Vite
        // always allows IPs + localhost; this only adds named hosts, so it's
        // inert for normal local dev. Lets `make web PUBLIC_HOST=<name>` work
        // when reached at https://<host>.<tailnet>.ts.net.
        allowedHosts: [".ts.net", ...(process.env.VITE_ALLOWED_HOSTS?.split(",").filter(Boolean) ?? [])],
        // Compile every dashboard page at startup, so no first click waits
        // on the dev server.
        warmup: {
            clientFiles: [
                "./src/main.tsx",
                "./src/router.tsx",
                "./src/app/app/**/page.tsx",
                "./src/app/app/**/layout.tsx",
            ],
        },
    },
}));
