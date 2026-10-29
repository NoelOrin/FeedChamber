/**
 * 代码块的印刷面样式（`CodeBlock` 的外壳与懒加载的高亮块共用同一份，避免两套代码块视觉）。
 * 单独成模块是因为高亮块必须保持懒加载：`CodeBlock` 不能静态 import 它。
 *
 * 取值来自设计文档 §4.7「代码块：color-mix(text 6%, card) 底、1px 边框、圆角 6px」，
 * 与 `components/ui/prose.tsx` 的 `[&_pre]` 完全一致。
 */
export const CODE_PRE_CLASS =
	'm-0 overflow-x-auto rounded-md border border-border bg-muted p-3 text-[13px] leading-relaxed text-foreground';
