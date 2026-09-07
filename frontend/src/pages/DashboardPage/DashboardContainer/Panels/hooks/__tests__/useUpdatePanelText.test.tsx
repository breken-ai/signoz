import { renderHook } from '@testing-library/react';

import { useUpdatePanelText } from '../useUpdatePanelText';

const patchAsync = jest.fn<Promise<unknown>, [unknown]>(() =>
	Promise.resolve(undefined),
);
const showErrorModal = jest.fn();
let store = { dashboardId: 'dash-1', isEditable: true };

jest.mock('../../../hooks/useOptimisticPatch', () => ({
	useOptimisticPatch: (): unknown => ({ patchAsync }),
}));
jest.mock('providers/ErrorModalProvider', () => ({
	useErrorModal: (): unknown => ({ showErrorModal }),
}));
jest.mock('../../../store/useDashboardStore', () => ({
	useDashboardStore: (select: (s: typeof store) => unknown): unknown =>
		select(store),
}));

describe('useUpdatePanelText', () => {
	beforeEach(() => {
		jest.clearAllMocks();
		store = { dashboardId: 'dash-1', isEditable: true };
	});

	it('patches the panel body', () => {
		const { result } = renderHook(() => useUpdatePanelText('p1'));

		result.current?.('- [x] done');

		expect(patchAsync).toHaveBeenCalledWith([
			{
				op: 'add',
				path: '/spec/panels/p1/spec/plugin/spec/text',
				value: '- [x] done',
			},
		]);
	});

	it('gives no callback when the viewer cannot edit', () => {
		store = { dashboardId: 'dash-1', isEditable: false };

		const { result } = renderHook(() => useUpdatePanelText('p1'));

		expect(result.current).toBeUndefined();
	});

	it('gives no callback outside a dashboard', () => {
		store = { dashboardId: '', isEditable: true };

		const { result } = renderHook(() => useUpdatePanelText('p1'));

		expect(result.current).toBeUndefined();
	});

	it('surfaces a failed save', async () => {
		const failure = new Error('locked');
		patchAsync.mockRejectedValueOnce(failure);
		const { result } = renderHook(() => useUpdatePanelText('p1'));

		result.current?.('- [x] done');
		await Promise.resolve();

		expect(showErrorModal).toHaveBeenCalledWith(failure);
	});
});
