'use client';

/**
 * RSC 预取后在页面里包一层水合边界。直接 import @tanstack/react-query 的
 * HydrationBoundary 到服务端组件会在 SSR 期就调用 useQueryClient（此时还没有
 * Provider），必须经这个 'use client' 边界转一道。
 */
export { HydrationBoundary } from '@tanstack/react-query';
