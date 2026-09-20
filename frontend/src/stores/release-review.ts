import { create } from 'zustand';
import {
  getReleaseBoard, judgeReleaseReview, listReleaseReviews, openReleaseReview, withdrawReleaseReview,
  type ReleaseVerdict,
} from '../api/release-review';
import { ApiError } from '../api/client';
import type { HeatReleaseReview, PageMeta, ReleasePairingView } from '../types/domain';

interface ReleaseReviewState {
  reviews: HeatReleaseReview[];
  meta: PageMeta;
  board: ReleasePairingView[];
  loading: boolean;
  error: string;
  loadReviews: (page?: number, pageSize?: number, search?: string, status?: string) => Promise<void>;
  loadBoard: () => Promise<void>;
  openReview: (heatCode: string, name: string, description?: string) => Promise<void>;
  judge: (id: number, target: ReleaseVerdict, expectedVersion: number, reason?: string) => Promise<void>;
  withdraw: (id: number) => Promise<void>;
  clearError: () => void;
}

function message(error: unknown): string {
  return error instanceof ApiError ? error.message : error instanceof Error ? error.message : String(error);
}

export const useReleaseReviewStore = create<ReleaseReviewState>((set) => ({
  reviews: [],
  meta: { page: 1, pageSize: 20, total: 0 },
  board: [],
  loading: false,
  error: '',
  clearError: () => set({ error: '' }),

  loadReviews: async (page = 1, pageSize = 20, search = '', status = '') => {
    set({ loading: true, error: '' });
    try {
      const result = await listReleaseReviews(page, pageSize, search, status);
      set({
        reviews: result.data,
        meta: result.meta || { page, pageSize, total: result.data.length },
        loading: false,
      });
    } catch (error) {
      set({ error: message(error), loading: false });
    }
  },

  loadBoard: async () => {
    set({ loading: true, error: '' });
    try {
      const result = await getReleaseBoard();
      set({ board: result.data, loading: false });
    } catch (error) {
      set({ error: message(error), loading: false });
    }
  },

  openReview: async (heatCode, name, description = '') => {
    set({ loading: true, error: '' });
    try {
      await openReleaseReview(heatCode, name, description);
      set({ loading: false });
    } catch (error) {
      set({ error: message(error), loading: false });
      throw error;
    }
  },

  judge: async (id, target, expectedVersion, reason = '') => {
    set({ loading: true, error: '' });
    try {
      await judgeReleaseReview(id, target, expectedVersion, reason);
      set({ loading: false });
    } catch (error) {
      set({ error: message(error), loading: false });
      throw error;
    }
  },

  withdraw: async (id) => {
    set({ loading: true, error: '' });
    try {
      await withdrawReleaseReview(id);
      set({ loading: false });
    } catch (error) {
      set({ error: message(error), loading: false });
      throw error;
    }
  },
}));
