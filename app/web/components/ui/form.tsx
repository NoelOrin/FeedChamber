'use client';

import { Slot } from '@radix-ui/react-slot';
import type * as React from 'react';
import { createContext, useContext, useId } from 'react';
import {
	Controller,
	type ControllerProps,
	type FieldPath,
	type FieldValues,
	FormProvider,
	useFormContext,
} from 'react-hook-form';

import { Label } from '~/components/ui/label';
import { cn } from '~/lib/utils';

/*
 * 表单原语（react-hook-form 适配层，shadcn 基准）。
 *
 * 替代 antd Form 的三件事：
 *   1. Form.Item 的 label + 必填星号 + 错误文案布局  → FormField / FormItem / FormLabel / FormMessage
 *   2. rules 数组校验                               → zod schema（resolver 挂在 useForm 上）
 *   3. form.validateFields / setFieldsValue         → RHF 的 handleSubmit / reset / setValue
 *
 * 错误文案用 aria-describedby 关联，错误态给控件打 aria-invalid，
 * Input/Textarea 已内置 aria-[invalid=true] 的红色描边。
 */

const Form = FormProvider;

type FormFieldContextValue<
	TFieldValues extends FieldValues = FieldValues,
	TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> = { name: TName };

const FormFieldContext = createContext<FormFieldContextValue>({} as FormFieldContextValue);

export function FormField<
	TFieldValues extends FieldValues = FieldValues,
	TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>({ ...props }: ControllerProps<TFieldValues, TName>) {
	return (
		<FormFieldContext.Provider value={{ name: props.name }}>
			<Controller {...props} />
		</FormFieldContext.Provider>
	);
}

export function useFormField() {
	const fieldContext = useContext(FormFieldContext);
	const itemContext = useContext(FormItemContext);
	const { getFieldState, formState } = useFormContext();

	const fieldState = getFieldState(fieldContext.name, formState);
	if (!fieldContext) throw new Error('useFormField 必须在 <FormField> 内使用');

	const { id, formItemId, formDescriptionId, formMessageId } = itemContext;

	return {
		id,
		name: fieldContext.name,
		formItemId,
		formDescriptionId,
		formMessageId,
		...fieldState,
	};
}

type FormItemContextValue = { id: string; formItemId: string; formDescriptionId: string; formMessageId: string };

const FormItemContext = createContext<FormItemContextValue>({} as FormItemContextValue);

export function FormItem({ className, ...props }: React.ComponentProps<'div'>) {
	const id = useId();
	const value = {
		id,
		formItemId: `${id}-form-item`,
		formDescriptionId: `${id}-form-item-description`,
		formMessageId: `${id}-form-item-message`,
	};

	return (
		<FormItemContext.Provider value={value}>
			<div data-slot="form-item" className={cn('flex flex-col gap-1.5', className)} {...props} />
		</FormItemContext.Provider>
	);
}

export function FormLabel({ className, children, ...props }: React.ComponentProps<typeof Label>) {
	const { error, formItemId } = useFormField();
	return (
		<Label
			data-slot="form-label"
			data-error={!!error}
			className={cn('data-[error=true]:text-destructive', className)}
			htmlFor={formItemId}
			{...props}
		>
			{children}
		</Label>
	);
}

export function FormControl({ ...props }: React.ComponentProps<typeof Slot>) {
	const { error, formItemId, formDescriptionId, formMessageId } = useFormField();
	return (
		<Slot
			data-slot="form-control"
			id={formItemId}
			aria-describedby={error ? `${formDescriptionId} ${formMessageId}` : formDescriptionId}
			aria-invalid={!!error}
			{...props}
		/>
	);
}

export function FormDescription({ className, ...props }: React.ComponentProps<'p'>) {
	const { formDescriptionId } = useFormField();
	return (
		<p
			data-slot="form-description"
			id={formDescriptionId}
			className={cn('text-[12px] text-muted-foreground', className)}
			{...props}
		/>
	);
}

export function FormMessage({ className, children, ...props }: React.ComponentProps<'p'>) {
	const { error, formMessageId } = useFormField();
	const body = error ? String(error?.message ?? '') : children;
	if (!body) return null;
	return (
		<p data-slot="form-message" id={formMessageId} className={cn('text-[12px] text-destructive', className)} {...props}>
			{body}
		</p>
	);
}

export { Form };
