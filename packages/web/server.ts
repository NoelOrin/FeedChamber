import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import express from 'express'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const root = __dirname

async function createServer() {
  const app = express()

  let vite: any = null

  if (process.env.NODE_ENV !== 'production') {
    const { createServer: createViteServer } = await import('vite')
    vite = await createViteServer({
      server: { middlewareMode: true },
      appType: 'custom',
    })
    app.use(vite.middlewares)
  } else {
    app.use(express.static(path.join(root, 'dist/client')))
  }

  app.use('{*path}', async (req, res) => {
    const url = req.originalUrl

    try {
      let template: string
      let render: any

      if (vite) {
        template = fs.readFileSync(path.join(root, 'index.html'), 'utf-8')
        template = await vite.transformIndexHtml(url, template)
        const mod = await vite.ssrLoadModule('/src/entry-server.tsx')
        render = mod.render
      } else {
        template = fs.readFileSync(
          path.join(root, 'dist/client/index.html'),
          'utf-8'
        )
        const mod = await import('./dist/server/entry-server.js')
        render = mod.render
      }

      const { html: appHtml } = await render(url)

      const html = template.replace('<!--app-html-->', appHtml)

      res.status(200).set({ 'Content-Type': 'text/html' }).end(html)
    } catch (e: any) {
      if (vite) {
        vite.ssrFixStacktrace(e)
      }
      console.error(e)
      res.status(500).end(e.message)
    }
  })

  const port = process.env.PORT || 3000
  app.listen(port, () => {
    console.log(`Server running at http://localhost:${port}`)
  })
}

createServer()
