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

export const getUploadedFiles = (params: {
  category?: string;
  p?: number;
}): Promise<FileListResponse> => {
  return api.get('/api/upload-management/files', { params });
};

export const deleteFile = (path: string): Promise<{ success: boolean; message: string }> => {
  return api.post('/api/upload-management/delete', { path });
};

export const batchDeleteFiles = (paths: string[]): Promise<{
  success: boolean;
  deleted: number;
  failed: number;
}> => {
  return api.post('/api/upload-management/batch-delete', { paths });
};

export const getUploadStats = (): Promise<StatsResponse> => {
  return api.get('/api/upload-management/stats');
};

export const cleanOldFiles = (params: {
  category: string;
  days: number;
}): Promise<{
  success: boolean;
  deleted: number;
  size: number;
  message: string;
}> => {
  return api.post('/api/upload-management/clean', params);
};
