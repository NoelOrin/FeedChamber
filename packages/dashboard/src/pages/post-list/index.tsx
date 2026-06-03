import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Dropdown, Modal, message, Space, Tag, Typography } from 'antd';
import React, { useRef } from 'react';
import { categoryService, postService } from '@/services/orinforum';
import CreateForm from './components/CreateForm';
import UpdateForm from './components/UpdateForm';

const { Text } = Typography;

const PostList: React.FC = () => {
  const actionRef = useRef<ActionType | null>(null);
  const queryClient = useQueryClient();

  const [messageApi, contextHolder] = message.useMessage();

  const { data: categoryData } = useQuery({
    queryKey: ['orinforum', 'categories'],
    queryFn: () => categoryService.list(),
  });

  const categories = categoryData?.data || [];

  const { mutate: deletePost, isPending: deleteLoading } = useMutation({
    mutationFn: (id: string) => postService.remove(id),
    onSuccess: () => {
      messageApi.success('Deleted successfully');
      queryClient.invalidateQueries({ queryKey: ['orinforum', 'posts'] });
      actionRef.current?.reloadAndRest?.();
    },
    onError: () => {
      messageApi.error('Delete failed');
    },
  });

  const { mutate: togglePin, isPending: pinLoading } = useMutation({
    mutationFn: (id: string) => postService.togglePin(id),
    onSuccess: () => {
      messageApi.success('Pin status updated');
      queryClient.invalidateQueries({ queryKey: ['orinforum', 'posts'] });
      actionRef.current?.reloadAndRest?.();
    },
    onError: () => {
      messageApi.error('Pin failed');
    },
  });

  const { mutate: toggleLock, isPending: lockLoading } = useMutation({
    mutationFn: (id: string) => postService.toggleLock(id),
    onSuccess: () => {
      messageApi.success('Lock status updated');
      queryClient.invalidateQueries({ queryKey: ['orinforum', 'posts'] });
      actionRef.current?.reloadAndRest?.();
    },
    onError: () => {
      messageApi.error('Lock failed');
    },
  });

  const intl = useIntl();

  const columns: ProColumns<API.PostListItem>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 80,
      copyable: true,
      ellipsis: true,
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.post.title',
        defaultMessage: 'Title',
      }),
      dataIndex: 'title',
      copyable: true,
      ellipsis: true,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.title}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {record.content_digest}
          </Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.post.author',
        defaultMessage: 'Author',
      }),
      dataIndex: 'author_name',
      width: 120,
      hideInSearch: true,
      render: (_, record) => (
        <Space>
          {record.author_avatar && (
            <img
              src={record.author_avatar}
              alt={record.author_name}
              style={{ width: 24, height: 24, borderRadius: '50%' }}
            />
          )}
          <Text>{record.author_name}</Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.post.category',
        defaultMessage: 'Category',
      }),
      dataIndex: 'category_name',
      width: 100,
      hideInSearch: true,
      render: (_, record) => <Tag color="blue">{record.category_name}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.post.stats',
        defaultMessage: 'Stats',
      }),
      dataIndex: 'stats',
      width: 160,
      hideInSearch: true,
      render: (_, record) => (
        <Space size="small" wrap>
          <Text type="secondary" style={{ fontSize: 12 }}>
            👁 {record.view_count}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            💬 {record.comment_count}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            ❤️ {record.like_count}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            ⭐ {record.favorite_count}
          </Text>
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.post.status',
        defaultMessage: 'Status',
      }),
      dataIndex: 'status',
      width: 120,
      hideInSearch: true,
      render: (_, record) => (
        <Space wrap>
          {record.pinned && <Tag color="gold">Pinned</Tag>}
          {record.locked && <Tag color="red">Locked</Tag>}
          {record.status === 'published' && <Tag color="green">Published</Tag>}
          {record.status === 'draft' && <Tag color="default">Draft</Tag>}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({
        id: 'pages.post.created',
        defaultMessage: 'Created',
      }),
      dataIndex: 'created_at',
      width: 150,
      valueType: 'dateTime',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.post.option',
        defaultMessage: 'Option',
      }),
      valueType: 'option',
      width: 200,
      render: (_, record) => [
        <UpdateForm
          key="edit"
          trigger={
            <a>
              <FormattedMessage id="pages.post.edit" defaultMessage="Edit" />
            </a>
          }
          values={record}
          categories={categories}
          onOk={() => actionRef.current?.reload()}
        />,
        <Dropdown
          key="more"
          menu={{
            items: [
              {
                key: 'pin',
                label: record.pinned ? 'Unpin' : 'Pin',
                onClick: () => togglePin(record.id),
              },
              {
                key: 'lock',
                label: record.locked ? 'Unlock' : 'Lock',
                onClick: () => toggleLock(record.id),
              },
              { type: 'divider' },
              {
                key: 'delete',
                label: 'Delete',
                danger: true,
                onClick: () => {
                  Modal.confirm({
                    title: 'Delete Post',
                    content: 'Are you sure you want to delete this post?',
                    onOk: () => deletePost(record.id),
                  });
                },
              },
            ],
          }}
        >
          <a>
            <FormattedMessage id="pages.post.more" defaultMessage="More" />
          </a>
        </Dropdown>,
      ],
    },
  ];

  return (
    <PageContainer>
      {contextHolder}
      <ProTable<API.PostListItem, API.PostListParams>
        headerTitle={intl.formatMessage({
          id: 'pages.post.title',
          defaultMessage: 'Posts',
        })}
        actionRef={actionRef}
        rowKey="id"
        search={{
          labelWidth: 'auto',
        }}
        toolBarRender={() => [
          <CreateForm
            key="create"
            categories={categories}
            reload={() => actionRef.current?.reload()}
          />,
        ]}
        request={async (params) => {
          const response = await postService.list({
            page: params.current,
            pageSize: params.pageSize,
            categoryName: params.category_name,
            status: params.status,
          });
          return {
            data: response.data || [],
            success: response.code === 0,
            total: response.pagination?.total || 0,
          };
        }}
        columns={columns}
      />
    </PageContainer>
  );
};

export default PostList;
