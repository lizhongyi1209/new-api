/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useEffect, useState } from 'react';
import { Button, Card, Col, Form, Modal, Row, Select, Spin, Table, Tag, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';

const { Text } = Typography;

const UploadManagement = () => {
  const [loading, setLoading] = useState(false);
  const [files, setFiles] = useState([]);
  const [stats, setStats] = useState(null);
  const [selectedCategory, setSelectedCategory] = useState('');
  const [selectedFiles, setSelectedFiles] = useState([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [cleanDialogVisible, setCleanDialogVisible] = useState(false);
  const [cleanDays, setCleanDays] = useState(90);

  const loadFiles = async () => {
    setLoading(true);
    try {
      const params = { p: page };
      if (selectedCategory) {
        params.category = selectedCategory;
      }
      const res = await API.get('/api/upload-management/files', { params });
      setFiles(res.data.data || []);
      setTotal(res.data.total || 0);
    } catch (error) {
      showError(error.message || '加载文件列表失败');
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const res = await API.get('/api/upload-management/stats');
      setStats(res.data.data);
    } catch (error) {
      showError(error.message || '加载统计信息失败');
    }
  };

  useEffect(() => {
    loadFiles();
  }, [selectedCategory, page]);

  useEffect(() => {
    loadStats();
  }, []);

  const handleDelete = async (path) => {
    if (!confirm('确定要删除这个文件吗？')) {
      return;
    }
    try {
      await API.post('/api/upload-management/delete', { path });
      showSuccess('删除成功');
      loadFiles();
      loadStats();
    } catch (error) {
      showError(error.message || '删除失败');
    }
  };

  const handleBatchDelete = async () => {
    if (selectedFiles.length === 0) {
      showError('请先选择要删除的文件');
      return;
    }
    if (!confirm(`确定要删除选中的 ${selectedFiles.length} 个文件吗？`)) {
      return;
    }
    try {
      const result = await API.post('/api/upload-management/batch-delete', { paths: selectedFiles });
      showSuccess(`删除成功 ${result.data.deleted} 个文件，失败 ${result.data.failed} 个`);
      setSelectedFiles([]);
      loadFiles();
      loadStats();
    } catch (error) {
      showError(error.message || '批量删除失败');
    }
  };

  const handleCleanOld = async () => {
    if (selectedCategory === 'elements') {
      showError('不能自动清理 elements 目录');
      return;
    }
    if (!selectedCategory) {
      showError('请先选择一个目录');
      return;
    }
    try {
      const result = await API.post('/api/upload-management/clean', {
        category: selectedCategory,
        days: cleanDays,
      });
      showSuccess(result.data.message);
      setCleanDialogVisible(false);
      loadFiles();
      loadStats();
    } catch (error) {
      showError(error.message || '清理失败');
    }
  };

  const formatFileSize = (bytes) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const formatDate = (timestamp) => {
    return new Date(timestamp * 1000).toLocaleString('zh-CN');
  };

  const columns = [
    {
      title: '预览',
      dataIndex: 'url',
      key: 'preview',
      width: 100,
      render: (url, record) =>
        record.is_image ? (
          <img src={url} alt={record.name} style={{ width: 64, height: 64, objectFit: 'cover' }} />
        ) : (
          <div
            style={{
              width: 64,
              height: 64,
              background: '#f0f0f0',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <Text type="secondary" size="small">
              文件
            </Text>
          </div>
        ),
    },
    {
      title: '文件名',
      dataIndex: 'name',
      key: 'name',
      render: (name, record) => (
        <a href={record.url} target="_blank" rel="noopener noreferrer">
          {name}
        </a>
      ),
    },
    {
      title: '目录',
      dataIndex: 'category',
      key: 'category',
      width: 120,
      render: (category) => {
        const colorMap = {
          elements: 'purple',
          uploads: 'green',
          temp: 'grey',
        };
        return (
          <Tag color={colorMap[category] || 'grey'}>
            {category}
            {category === 'elements' && ' 🔒'}
          </Tag>
        );
      },
    },
    {
      title: '大小',
      dataIndex: 'size',
      key: 'size',
      width: 100,
      render: (size) => formatFileSize(size),
    },
    {
      title: '修改时间',
      dataIndex: 'mod_time',
      key: 'mod_time',
      width: 180,
      render: (time) => formatDate(time),
    },
    {
      title: '操作',
      key: 'action',
      width: 80,
      render: (_, record) => (
        <Button type="danger" size="small" onClick={() => handleDelete(record.path)}>
          删除
        </Button>
      ),
    },
  ];

  const rowSelection = {
    selectedRowKeys: selectedFiles,
    onChange: (selectedRowKeys) => {
      setSelectedFiles(selectedRowKeys);
    },
  };

  return (
    <div style={{ padding: 20 }}>
      <Typography.Title heading={2}>上传管理</Typography.Title>

      {stats && (
        <Row gutter={16} style={{ marginBottom: 20 }}>
          <Col span={6}>
            <Card style={{ background: '#e6f7ff', textAlign: 'center' }}>
              <Text type="secondary">总文件数</Text>
              <Typography.Title heading={3}>{stats.total.count}</Typography.Title>
              <Text size="small">{formatFileSize(stats.total.size)}</Text>
            </Card>
          </Col>
          <Col span={6}>
            <Card style={{ background: '#f6ffed', textAlign: 'center' }}>
              <Text type="secondary">普通上传</Text>
              <Typography.Title heading={3}>{stats.uploads.count}</Typography.Title>
              <Text size="small">{formatFileSize(stats.uploads.size)}</Text>
            </Card>
          </Col>
          <Col span={6}>
            <Card style={{ background: '#f9f0ff', textAlign: 'center' }}>
              <Text type="secondary">可灵元素 🔒</Text>
              <Typography.Title heading={3}>{stats.elements.count}</Typography.Title>
              <Text size="small">{formatFileSize(stats.elements.size)}</Text>
            </Card>
          </Col>
          <Col span={6}>
            <Card style={{ background: '#f5f5f5', textAlign: 'center' }}>
              <Text type="secondary">临时文件</Text>
              <Typography.Title heading={3}>{stats.temp.count}</Typography.Title>
              <Text size="small">{formatFileSize(stats.temp.size)}</Text>
            </Card>
          </Col>
        </Row>
      )}

      <Card style={{ marginBottom: 20 }}>
        <div style={{ display: 'flex', gap: 10, alignItems: 'center', flexWrap: 'wrap' }}>
          <Select
            placeholder="全部目录"
            style={{ width: 200 }}
            value={selectedCategory}
            onChange={(value) => {
              setSelectedCategory(value);
              setPage(1);
            }}
          >
            <Select.Option value="">全部目录</Select.Option>
            <Select.Option value="uploads">普通上传 (uploads)</Select.Option>
            <Select.Option value="elements">可灵元素 (elements) 🔒</Select.Option>
            <Select.Option value="temp">临时文件 (temp)</Select.Option>
          </Select>

          <Button
            type="danger"
            disabled={selectedFiles.length === 0}
            onClick={handleBatchDelete}
          >
            删除选中 ({selectedFiles.length})
          </Button>

          {selectedCategory && selectedCategory !== 'elements' && (
            <Button type="warning" onClick={() => setCleanDialogVisible(true)}>
              清理旧文件
            </Button>
          )}

          <Button onClick={loadFiles}>刷新</Button>
        </div>
      </Card>

      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={files}
          rowKey="path"
          rowSelection={rowSelection}
          pagination={{
            currentPage: page,
            pageSize: 50,
            total: total,
            onPageChange: setPage,
          }}
        />
      </Spin>

      <Modal
        title="清理旧文件"
        visible={cleanDialogVisible}
        onCancel={() => setCleanDialogVisible(false)}
        onOk={handleCleanOld}
      >
        <Form>
          <Text>
            将删除 <strong>{selectedCategory}</strong> 目录中超过指定天数的文件
          </Text>
          <Form.Input
            field="days"
            label="保留天数"
            type="number"
            value={cleanDays}
            onChange={(value) => setCleanDays(parseInt(value) || 90)}
            min={1}
            style={{ marginTop: 20 }}
          />
          <Text type="tertiary" size="small">
            删除超过此天数的文件
          </Text>
        </Form>
      </Modal>
    </div>
  );
};

export default UploadManagement;
