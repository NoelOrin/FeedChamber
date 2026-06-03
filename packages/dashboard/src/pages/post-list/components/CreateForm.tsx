import { PlusOutlined } from '@ant-design/icons';
import {
  type ActionType,
  ModalForm,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, message } from 'antd';
import type { FC } from 'react';
import { postService } from '@/services/orinforum';

interface CreateFormProps {
  reload?: ActionType['reload'];
  categories: API.Category[];
}

const CreateForm: FC<CreateFormProps> = (props) => {
  const { reload, categories } = props;
  const [messageApi, contextHolder] = message.useMessage();
  const queryClient = useQueryClient();

  const intl = useIntl();

  const { mutateAsync: createPost, isPending: loading } = useMutation({
    mutationFn: (data: {
      title: string;
      content: string;
      categoryId: string;
    }) => postService.create(data),
    onSuccess: () => {
      messageApi.success('Post created successfully');
      queryClient.invalidateQueries({ queryKey: ['orinforum', 'posts'] });
      reload?.();
    },
    onError: () => {
      messageApi.error('Failed to create post');
    },
  });

  return (
    <>
      {contextHolder}
      <ModalForm
        title={intl.formatMessage({
          id: 'pages.post.create',
          defaultMessage: 'Create Post',
        })}
        trigger={
          <Button type="primary" icon={<PlusOutlined />}>
            <FormattedMessage id="pages.post.new" defaultMessage="New Post" />
          </Button>
        }
        width="600px"
        modalProps={{ okButtonProps: { loading } }}
        onFinish={async (values) => {
          try {
            await createPost({
              title: values.title,
              content: values.content,
              categoryId: values.categoryId,
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
          rules={[
            {
              required: true,
              message: 'Please enter post title',
            },
          ]}
        />
        <ProFormSelect
          name="categoryId"
          label={intl.formatMessage({
            id: 'pages.post.form.category',
            defaultMessage: 'Category',
          })}
          width="md"
          rules={[{ required: true, message: 'Please select category' }]}
          options={categories.map((c) => ({ label: c.name, value: c.id }))}
        />
        <ProFormTextArea
          name="content"
          label={intl.formatMessage({
            id: 'pages.post.form.content',
            defaultMessage: 'Content',
          })}
          width="lg"
          rows={8}
          rules={[
            {
              required: true,
              message: 'Please enter post content',
            },
          ]}
        />
      </ModalForm>
    </>
  );
};

export default CreateForm;
