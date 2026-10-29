/**
 * 后台错误页。
 *
 * 此前这里是与 `components/error/error-fallback.tsx` 平行的第二份实现
 * （141 行 vs 131 行），版式、重试策略、DEV 详情都不一致。2026-10-28 合并为
 * 单一实现：这里只做再导出，并把后台的默认跳转目标注入进去。
 *
 * 之所以仍保留这个文件而不是直接改 import：`components/admin/error/index.ts`
 * 与 `app/admin/error.tsx` 都按默认导出引用它，保留再导出层可以让合并后的
 * 差异集中在组件本体，后台调用方零改动。
 */
import { ErrorFallback, type ErrorFallbackProps, type ErrorFallbackVariant } from '~/components/error/error-fallback';

export type { ErrorFallbackProps, ErrorFallbackVariant };
export default ErrorFallback;
