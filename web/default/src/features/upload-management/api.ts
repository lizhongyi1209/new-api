import { api } from '@/lib/api';

export interface FileInfo {
  path: string;
  name: string;
  size: number;
  mod_time: number;
  category: string;
  url: string;
  thumbnail_url: string;
  is_image: boolean;
}

export interface FileListResponse {
  success: boolean;
  data: FileInfo[];
  total: number;
  page: number;
  stats: {
    uploads: { count: number; size: number };
    elements: { count: number; size: number };
    temp: { count: number; size: number };
  };
}

export interface StatsResponse {
  success: boolean;
  data: {
    uploads: { count: number; size: number };
    elements: { count: number; size: number };
    temp: { count: number; size: number };
    total: { count: number; size: number };
  };
}

export const getUploadedFiles = async (params: {
  category?: string;
  p?: number;
}): Promise<FileListResponse> => {
  const res = await api.get('/api/upload-management/files', { params });
  return res.data;
};

export const deleteFile = async (path: string): Promise<{ success: boolean; message: string }> => {
  const res = await api.post('/api/upload-management/delete', { path });
  return res.data;
};

export const batchDeleteFiles = async (paths: string[]): Promise<{
  success: boolean;
  deleted: number;
  failed: number;
}> => {
  const res = await api.post('/api/upload-management/batch-delete', { paths });
  return res.data;
};

export const getUploadStats = async (): Promise<StatsResponse> => {
  const res = await api.get('/api/upload-management/stats');
  return res.data;
};

export const cleanOldFiles = async (params: {
  category: string;
  days: number;
}): Promise<{
  success: boolean;
  deleted: number;
  size: number;
  message: string;
}> => {
  const res = await api.post('/api/upload-management/clean', params);
  return res.data;
};
