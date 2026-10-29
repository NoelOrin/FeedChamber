'use client';

import {
	type ColumnDef,
	type ColumnFiltersState,
	flexRender,
	getCoreRowModel,
	getFilteredRowModel,
	getSortedRowModel,
	type SortingState,
	type Table as TanstackTable,
	useReactTable,
} from '@tanstack/react-table';
import type * as React from 'react';
import { useState } from 'react';

import { Checkbox } from '~/components/ui/checkbox';
import { EmptyState } from '~/components/ui/empty-state';
import { Loading } from '~/components/ui/loading';
import { Pagination } from '~/components/ui/pagination';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table';
import { ChevronDown, ChevronsUpDown, ChevronUp } from '~/lib/icons';
import { cn } from '~/lib/utils';

/*
 * 数据表格（TanStack Table + 上面的 Table 壳）。
 *
 * 替代 ProTable。与 ProTable 的关键差别：**取数不在这里**。
 * ProTable 的 `request` 把「取数 + 分页 + 排序 + 加载态」揉在一起；
 * 这里只做展示，调用方用 useQuery 取数（与主站一致），把 rows/total/page 传进来。
 * 这样缓存、失效、乐观更新都归 TanStack Query，表格保持无状态、可测。
 *
 * 排序模式：
 *   - 'client'：全量数据在前端排（数据量小，如角色/标签）
 *   - 'server'：只把排序意图通过 onSortingChange 抛给调用方（数据量大，如帖子/用户）
 */

export type DataTableSorting = SortingState;

export interface DataTableProps<TData, TValue> {
	columns: ColumnDef<TData, TValue>[];
	data: TData[];
	loading?: boolean;
	/** 空数据文案 */
	emptyText?: string;
	/** 行选择：传入即显示复选框列 */
	rowSelection?: Record<string, boolean>;
	onRowSelectionChange?: (selection: Record<string, boolean>) => void;
	/** 行唯一键，默认 row.id */
	getRowId?: (row: TData) => string;
	/** 服务端分页：传入即显示分页条 */
	pagination?: {
		page: number;
		pageSize: number;
		total: number;
		onChange: (page: number) => void;
	};
	/** 排序模式，默认 client */
	sortingMode?: 'client' | 'server';
	sorting?: DataTableSorting;
	onSortingChange?: (sorting: DataTableSorting) => void;
	/** 表格上方工具条（搜索框 / 批量操作） */
	toolbar?: React.ReactNode;
	className?: string;
	/** 行点击 */
	onRowClick?: (row: TData) => void;
}

export function DataTable<TData, TValue>({
	columns,
	data,
	loading = false,
	emptyText = '暂无数据',
	rowSelection,
	onRowSelectionChange,
	getRowId,
	pagination,
	sortingMode = 'client',
	sorting: controlledSorting,
	onSortingChange,
	toolbar,
	className,
	onRowClick,
}: DataTableProps<TData, TValue>) {
	const [internalSorting, setInternalSorting] = useState<DataTableSorting>([]);
	const [columnFilters] = useState<ColumnFiltersState>([]);

	const sorting = controlledSorting ?? internalSorting;
	const handleSortingChange = (updater: DataTableSorting | ((prev: DataTableSorting) => DataTableSorting)) => {
		const next = typeof updater === 'function' ? updater(sorting) : updater;
		if (onSortingChange) onSortingChange(next);
		else setInternalSorting(next);
	};

	const table = useReactTable({
		data,
		columns,
		getRowId,
		state: {
			sorting,
			columnFilters,
			...(rowSelection ? { rowSelection } : {}),
		},
		onSortingChange: handleSortingChange,
		onRowSelectionChange: rowSelection
			? (updater) => {
					const next = typeof updater === 'function' ? updater(rowSelection) : updater;
					onRowSelectionChange?.(next);
				}
			: undefined,
		enableRowSelection: !!rowSelection,
		getCoreRowModel: getCoreRowModel(),
		...(sortingMode === 'client'
			? { getSortedRowModel: getSortedRowModel(), getFilteredRowModel: getFilteredRowModel() }
			: {}),
		manualPagination: true,
		manualSorting: sortingMode === 'server',
	});

	const selectedCount = rowSelection ? Object.values(rowSelection).filter(Boolean).length : 0;

	return (
		<div data-slot="data-table" className={cn('flex flex-col gap-3', className)}>
			{toolbar ? <div className="flex flex-wrap items-center gap-2">{toolbar}</div> : null}
			{selectedCount > 0 ? (
				<div className="flex items-center gap-2 rounded-md border border-border bg-muted/50 px-3 py-1.5 text-[13px] text-muted-foreground">
					已选 <span className="font-medium text-foreground tabular-nums">{selectedCount}</span> 项
				</div>
			) : null}

			<div className="overflow-hidden rounded-lg border border-border/60 bg-card">
				<Table>
					<TableHeader>
						{table.getHeaderGroups().map((headerGroup) => (
							<TableRow key={headerGroup.id} className="bg-muted/40 hover:bg-muted/40">
								{headerGroup.headers.map((header) => {
									const canSort = header.column.getCanSort();
									const sortDir = header.column.getIsSorted();
									return (
										<TableHead key={header.id} colSpan={header.colSpan}>
											{header.isPlaceholder ? null : canSort ? (
												<button
													type="button"
													onClick={header.column.getToggleSortingHandler()}
													className="inline-flex cursor-pointer items-center gap-1 text-inherit hover:text-foreground"
												>
													{flexRender(header.column.columnDef.header, header.getContext())}
													{sortDir === 'asc' ? (
														<ChevronUp className="size-3" />
													) : sortDir === 'desc' ? (
														<ChevronDown className="size-3" />
													) : (
														<ChevronsUpDown className="size-3 opacity-40" />
													)}
												</button>
											) : (
												flexRender(header.column.columnDef.header, header.getContext())
											)}
										</TableHead>
									);
								})}
							</TableRow>
						))}
					</TableHeader>
					<TableBody>
						{loading ? (
							<TableRow>
								<TableCell colSpan={columns.length} className="h-32">
									<Loading className="justify-center" />
								</TableCell>
							</TableRow>
						) : table.getRowModel().rows.length === 0 ? (
							<TableRow>
								<TableCell colSpan={columns.length} className="h-32">
									<EmptyState title={emptyText} className="border-0 bg-transparent py-0" />
								</TableCell>
							</TableRow>
						) : (
							table.getRowModel().rows.map((row) => (
								<TableRow
									key={row.id}
									data-state={row.getIsSelected() ? 'selected' : undefined}
									className={onRowClick ? 'cursor-pointer' : undefined}
									onClick={onRowClick ? () => onRowClick(row.original) : undefined}
								>
									{row.getVisibleCells().map((cell) => (
										<TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
									))}
								</TableRow>
							))
						)}
					</TableBody>
				</Table>
			</div>

			{pagination ? (
				<Pagination
					page={pagination.page}
					pageSize={pagination.pageSize}
					total={pagination.total}
					onChange={pagination.onChange}
					disabled={loading}
				/>
			) : null}
		</div>
	);
}

/** 行选择列的标准写法：`selectColumn()` 塞进 columns 数组首位。 */
export function selectColumn<TData>(): ColumnDef<TData> {
	return {
		id: 'select',
		header: ({ table }) => (
			<Checkbox
				checked={table.getIsAllPageRowsSelected() ? true : table.getIsSomePageRowsSelected() ? 'indeterminate' : false}
				onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
				aria-label="全选"
			/>
		),
		cell: ({ row }) => (
			<Checkbox
				checked={row.getIsSelected()}
				disabled={!row.getCanSelect()}
				onCheckedChange={(value) => row.toggleSelected(!!value)}
				aria-label="选择此行"
			/>
		),
		enableSorting: false,
		size: 36,
	};
}

export { flexRender, type TanstackTable, useReactTable };
