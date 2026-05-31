import { defineNitroConfig } from 'nitro/config'

const apiBase = process.env.VITE_API_BASE_URL ?? process.env.API_BASE_URL ?? 'http://localhost:8080'

export default defineNitroConfig({
  routeRules: {
    '/api/v1/**': { proxy: `${apiBase}/api/v1/**` },
  },
})
