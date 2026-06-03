import {
  ModalForm,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { FormattedMessage, useIntl } from '@umijs/max';
import { message } from 'antd';
import React from 'react';
import { postService } from '@/services/orinforum';

type UpdateFormProps = {
  trigger?: React.ReactElement<any>;
  onOk?: () => void;
  values: Partial<API.PostListItem>;
};

const UpdateForm: React.FC<UpdateFormProps> = (props) => {
  const { onOk, values, trigger } = props;
  const [messageApi, contextHolder] = message.useMessage();
  const queryClient = useQueryClient();

  const intl = useIntl();

  const { mutateAsync: updatePost, isPending: loading } = useMutation({
    mutationFn: (data: { title: string; content: string }) =>
      postService.update(values.id!, data),
    onSuccess: () => {
      messageApi.success('Post updated successfully');
      queryClient.invalidateQueries({ queryKey: ['orinforum', 'posts'] });
      onOk?.();
    },
    onError: () => {
      messageApi.error('Failed to update post');
    },
  });

  return (
    <>
      {contextHolder}
      <ModalForm
        title={intl.formatMessage({
          id: 'pages.post.edit',
          defaultMessage: 'Edit Post',
        })}
        trigger={trigger}
        width="600px"
        modalProps={{ okButtonProps: { loading } }}
        initialValues={{
          title: values.title,
        }}
        onFinish={async (formValues) => {
          try {
            await updatePost({
              title: formValues.title,
              content: formValues.content,
            });
            return true;
          } catch {
            return false;
          }
        }}
      >
        <ProFormText
          name="title"
          label={intl.formatMessage({
            id: 'pages.post.form.title',
            defaultMessage: 'Title',
          })}
          width="md"
          rules={[{ required: true, message: 'Please enter post title' }]}
        />
        <ProFormTextArea
          name="content"
          label={intl.formatMessage({
            id: 'pages.post.form.content',
            defaultMessage: 'Content',
          })}
          width="lg"
          rows={8}
          rules={[{ required: true, message: 'Please enter post content' }]}
        />
      </ModalForm>
    </>
  );
};

export default UpdateForm;
