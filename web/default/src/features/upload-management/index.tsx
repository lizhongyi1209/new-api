import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import {
  getUploadedFiles,
  deleteFile,
  batchDeleteFiles,
  getUploadStats,
  cleanOldFiles,
  type FileInfo,
  type StatsResponse,
} from './api';

export default function UploadManagement() {
  const { t } = useTranslation();
  const [files, setFiles] = useState<FileInfo[]>([]);
  const [stats, setStats] = useState<StatsResponse['data'] | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedCategory, setSelectedCategory] = useState<string>('');
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set());
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [cleanDays, setCleanDays] = useState(90);
  const [showCleanDialog, setShowCleanDialog] = useState(false);

  const loadFiles = async () => {
    setLoading(true);
    try {
      const response = await getUploadedFiles({
        category: selectedCategory || undefined,
        p: page,
      });
      setFiles(response.data || []);
      setTotal(response.total || 0);
    } catch (error: any) {
      toast.error(error.message || '加载文件列表失败');
      setFiles([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const response = await getUploadStats();
      setStats(response.data);
    } catch (error: any) {
      toast.error(error.message || '加载统计信息失败');
      setStats(null);
    }
  };

  useEffect(() => {
    loadFiles();
  }, [selectedCategory, page]);

  useEffect(() => {
    loadStats();
  }, []);

  const handleDelete = async (path: string) => {
    if (!confirm('确定要删除这个文件吗？')) {
      return;
    }

    try {
      await deleteFile(path);
      toast.success('删除成功');
      loadFiles();
      loadStats();
    } catch (error: any) {
      toast.error(error.message || '删除失败');
    }
  };

  const handleBatchDelete = async () => {
    if (selectedFiles.size === 0) {
      toast.error('请先选择要删除的文件');
      return;
    }

    if (!confirm(`确定要删除选中的 ${selectedFiles.size} 个文件吗？`)) {
      return;
    }

    try {
      const result = await batchDeleteFiles(Array.from(selectedFiles));
      toast.success(`删除成功 ${result.deleted} 个文件，失败 ${result.failed} 个`);
      setSelectedFiles(new Set());
      loadFiles();
      loadStats();
    } catch (error: any) {
      toast.error(error.message || '批量删除失败');
    }
  };

  const handleCleanOld = async () => {
    if (selectedCategory === 'elements') {
      toast.error('不能自动清理 elements 目录');
      return;
    }

    if (!selectedCategory) {
      toast.error('请先选择一个目录');
      return;
    }

    try {
      const result = await cleanOldFiles({
        category: selectedCategory,
        days: cleanDays,
      });
      toast.success(result.message);
      setShowCleanDialog(false);
      loadFiles();
      loadStats();
    } catch (error: any) {
      toast.error(error.message || '清理失败');
    }
  };

  const toggleFileSelection = (path: string) => {
    const newSelected = new Set(selectedFiles);
    if (newSelected.has(path)) {
      newSelected.delete(path);
    } else {
      newSelected.add(path);
    }
    setSelectedFiles(newSelected);
  };

  const toggleSelectAll = () => {
    if (selectedFiles.size === files.length) {
      setSelectedFiles(new Set());
    } else {
      setSelectedFiles(new Set(files.map((f) => f.path)));
    }
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const formatDate = (timestamp: number): string => {
    return new Date(timestamp * 1000).toLocaleString('zh-CN');
  };

  return (
    <div className="p-6 max-w-7xl mx-auto">
      <h1 className="text-3xl font-bold mb-6">素材管理</h1>

      {/* Statistics */}
      {stats && (
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          <div className="bg-blue-50 dark:bg-blue-900/20 p-4 rounded-lg">
            <div className="text-sm text-gray-600 dark:text-gray-400">总文件数</div>
            <div className="text-2xl font-bold">{stats.total.count}</div>
            <div className="text-xs text-gray-500">{formatFileSize(stats.total.size)}</div>
          </div>
          <div className="bg-green-50 dark:bg-green-900/20 p-4 rounded-lg">
            <div className="text-sm text-gray-600 dark:text-gray-400">普通上传</div>
            <div className="text-2xl font-bold">{stats.uploads.count}</div>
            <div className="text-xs text-gray-500">{formatFileSize(stats.uploads.size)}</div>
          </div>
          <div className="bg-purple-50 dark:bg-purple-900/20 p-4 rounded-lg">
            <div className="text-sm text-gray-600 dark:text-gray-400">可灵元素 🔒</div>
            <div className="text-2xl font-bold">{stats.elements.count}</div>
            <div className="text-xs text-gray-500">{formatFileSize(stats.elements.size)}</div>
          </div>
          <div className="bg-gray-50 dark:bg-gray-900/20 p-4 rounded-lg">
            <div className="text-sm text-gray-600 dark:text-gray-400">临时文件</div>
            <div className="text-2xl font-bold">{stats.temp.count}</div>
            <div className="text-xs text-gray-500">{formatFileSize(stats.temp.size)}</div>
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow p-4 mb-4">
        <div className="flex flex-wrap gap-4 items-center">
          <select
            className="px-4 py-2 border rounded-lg dark:bg-gray-700 dark:border-gray-600"
            value={selectedCategory}
            onChange={(e) => {
              setSelectedCategory(e.target.value);
              setPage(1);
            }}
          >
            <option value="">全部目录</option>
            <option value="uploads">普通上传 (uploads)</option>
            <option value="elements">可灵元素 (elements) 🔒</option>
            <option value="temp">临时文件 (temp)</option>
          </select>

          <button
            className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50"
            onClick={handleBatchDelete}
            disabled={selectedFiles.size === 0}
          >
            删除选中 ({selectedFiles.size})
          </button>

          {selectedCategory && selectedCategory !== 'elements' && (
            <button
              className="px-4 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700"
              onClick={() => setShowCleanDialog(true)}
            >
              清理旧文件
            </button>
          )}

          <button
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
            onClick={loadFiles}
          >
            刷新
          </button>
        </div>
      </div>

      {/* File List */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow">
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead className="bg-gray-50 dark:bg-gray-700">
              <tr>
                <th className="px-4 py-3 text-left">
                  <input
                    type="checkbox"
                    checked={files.length > 0 && selectedFiles.size === files.length}
                    onChange={toggleSelectAll}
                  />
                </th>
                <th className="px-4 py-3 text-left">预览</th>
                <th className="px-4 py-3 text-left">文件名</th>
                <th className="px-4 py-3 text-left">目录</th>
                <th className="px-4 py-3 text-left">大小</th>
                <th className="px-4 py-3 text-left">修改时间</th>
                <th className="px-4 py-3 text-left">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y dark:divide-gray-700">
              {loading ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-gray-500">
                    加载中...
                  </td>
                </tr>
              ) : files.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-gray-500">
                    暂无文件
                  </td>
                </tr>
              ) : (
                files.map((file) => (
                  <tr key={file.path} className="hover:bg-gray-50 dark:hover:bg-gray-700">
                    <td className="px-4 py-3">
                      <input
                        type="checkbox"
                        checked={selectedFiles.has(file.path)}
                        onChange={() => toggleFileSelection(file.path)}
                      />
                    </td>
                    <td className="px-4 py-3">
                      {file.is_image ? (
                        <img
                          src={file.thumbnail_url}
                          alt={file.name}
                          className="w-16 h-16 object-cover rounded"
                          loading="lazy"
                        />
                      ) : (
                        <div className="w-16 h-16 bg-gray-200 dark:bg-gray-600 rounded flex items-center justify-center">
                          <span className="text-xs text-gray-500">文件</span>
                        </div>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <a
                        href={file.url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-600 hover:underline truncate max-w-xs block"
                      >
                        {file.name}
                      </a>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`px-2 py-1 rounded text-xs ${
                          file.category === 'elements'
                            ? 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200'
                            : file.category === 'uploads'
                            ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                            : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200'
                        }`}
                      >
                        {file.category}
                        {file.category === 'elements' && ' 🔒'}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-sm">{formatFileSize(file.size)}</td>
                    <td className="px-4 py-3 text-sm text-gray-600 dark:text-gray-400">
                      {formatDate(file.mod_time)}
                    </td>
                    <td className="px-4 py-3">
                      <button
                        className="text-red-600 hover:text-red-800 text-sm"
                        onClick={() => handleDelete(file.path)}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {total > 50 && (
          <div className="px-4 py-3 border-t dark:border-gray-700 flex justify-between items-center">
            <div className="text-sm text-gray-600 dark:text-gray-400">
              共 {total} 个文件，第 {page} 页
            </div>
            <div className="flex gap-2">
              <button
                className="px-3 py-1 border rounded hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                disabled={page === 1}
              >
                上一页
              </button>
              <button
                className="px-3 py-1 border rounded hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50"
                onClick={() => setPage((p) => p + 1)}
                disabled={page * 50 >= total}
              >
                下一页
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Clean Old Files Dialog */}
      {showCleanDialog && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg p-6 max-w-md w-full">
            <h2 className="text-xl font-bold mb-4">清理旧文件</h2>
            <p className="text-gray-600 dark:text-gray-400 mb-4">
              将删除 <span className="font-bold">{selectedCategory}</span> 目录中超过指定天数的文件
            </p>
            <div className="mb-4">
              <label className="block text-sm font-medium mb-2">保留天数</label>
              <input
                type="number"
                className="w-full px-4 py-2 border rounded-lg dark:bg-gray-700 dark:border-gray-600"
                value={cleanDays}
                onChange={(e) => setCleanDays(parseInt(e.target.value) || 90)}
                min="1"
              />
              <p className="text-xs text-gray-500 mt-1">删除超过此天数的文件</p>
            </div>
            <div className="flex gap-2 justify-end">
              <button
                className="px-4 py-2 border rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700"
                onClick={() => setShowCleanDialog(false)}
              >
                取消
              </button>
              <button
                className="px-4 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700"
                onClick={handleCleanOld}
              >
                确认清理
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
