import { create, type StoreApi, type UseBoundStore } from 'zustand';
import { request } from '../api/client';
import type { BaseRecord, PageMeta } from '../types/domain';

export interface EntityState<T extends BaseRecord> {
  items: T[];
  meta: PageMeta;
  loading: boolean;
  error: string;
  load: (path: string, search?: string, page?: number, pageSize?: number) => Promise<void>;
  createRecord: (path: string, input: Record<string, unknown>) => Promise<void>;
  updateRecord: (path: string, item: T, input: Record<string, unknown>) => Promise<void>;
  transition: (path: string, item: T, status: string, reason: string) => Promise<void>;
  deleteRecord: (path: string, item: T) => Promise<void>;
  clearError: () => void;
}

export type EntityStore<T extends BaseRecord> = UseBoundStore<StoreApi<EntityState<T>>>;

function message(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

export function createEntityStore<T extends BaseRecord>(): EntityStore<T> {
  return create<EntityState<T>>((set, get) => ({
    items: [],
    meta: { page: 1, pageSize: 20, total: 0 },
    loading: false,
    error: '',
    clearError: () => set({ error: '' }),
    load: async (path, search = '', page = 1, pageSize = 20) => {
      set({ loading: true, error: '' });
      try {
        const result = await request<T[]>(`/${path}?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
        set({
          items: result.data,
          meta: result.meta || { page, pageSize, total: result.data.length },
          loading: false,
        });
      } catch (error) {
        set({ error: message(error), loading: false });
      }
    },
    createRecord: async (path, input) => {
      set({ loading: true, error: '' });
      try {
        await request<T>(`/${path}`, { method: 'POST', body: JSON.stringify(input) });
        await get().load(path, '', 1, get().meta.pageSize);
      } catch (error) {
        set({ error: message(error), loading: false });
        throw error;
      }
    },
    updateRecord: async (path, item, input) => {
      set({ loading: true, error: '' });
      try {
        await request<T>(`/${path}/${item.id}`, {
          method: 'PUT', body: JSON.stringify({ ...input, expectedVersion: item.version }),
        });
        await get().load(path, '', get().meta.page, get().meta.pageSize);
      } catch (error) {
        set({ error: message(error), loading: false });
        throw error;
      }
    },
    transition: async (path, item, status, reason) => {
      set({ loading: true, error: '' });
      try {
        await request<T>(`/${path}/${item.id}/transition`, {
          method: 'POST', body: JSON.stringify({ status, expectedVersion: item.version, reason }),
        });
        await get().load(path, '', get().meta.page, get().meta.pageSize);
      } catch (error) {
        set({ error: message(error), loading: false });
        throw error;
      }
    },
    deleteRecord: async (path, item) => {
      set({ loading: true, error: '' });
      try {
        await request<void>(`/${path}/${item.id}`, { method: 'DELETE' });
        await get().load(path, '', get().meta.page, get().meta.pageSize);
      } catch (error) {
        set({ error: message(error), loading: false });
        throw error;
      }
    },
  }));
}
