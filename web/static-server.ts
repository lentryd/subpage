// Workaround for https://github.com/oven-sh/bun/issues/18258 — CSS Modules are
// broken in `bun --hot index.html` dev mode, but work fine in `bun build`.
// Serves the `dist` output produced by `bun build --watch` with SPA fallback.
import { join } from 'node:path'

const dist = join(import.meta.dir, 'dist')
const port = 3000

Bun.serve({
    port,
    async fetch(req) {
        const url = new URL(req.url)
        let path = url.pathname === '/' ? '/index.html' : url.pathname

        let file = Bun.file(join(dist, path))
        if (!(await file.exists())) {
            file = Bun.file(join(dist, 'index.html'))
        }

        return new Response(file)
    },
})

console.log(`Static dev server → http://localhost:${port}/`)
