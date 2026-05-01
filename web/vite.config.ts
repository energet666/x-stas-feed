import tailwindcss from "@tailwindcss/vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), "");
	const apiTarget = env.VITE_API_TARGET || "http://localhost:8080";

	return {
		plugins: [svelte(), tailwindcss()],
		server: {
			host: "0.0.0.0",
			port: 5173,
			strictPort: true,
			proxy: {
				"/api": {
					target: apiTarget,
					changeOrigin: true,
				},
				"/media": {
					target: apiTarget,
					changeOrigin: true,
				},
			},
			allowedHosts: "all",
		},
	};
});
