import { beforeEach, describe, expect, it, vi } from 'vitest';
import { post } from '../client';
import { PermissionCode } from '../constants';
import { pluginService } from './plugin';

vi.mock('../client', () => ({ post: vi.fn() }));

const postMock = vi.mocked(post);

beforeEach(() => {
	postMock.mockReset();
});

describe('pluginService v2 contract', () => {
	it('list() reads the plugin list', () => {
		pluginService.list();
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/list', {});
	});

	it('listSources() calls post with the sources/list URL and an empty body', () => {
		pluginService.listSources();
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/sources/list', {});
	});

	it('createSource(body) posts to sources/create with the exact body', () => {
		const body = { name: 'official', kind: 'git', url: 'https://example.com/plugins.git' };
		pluginService.createSource(body);
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/sources/create', body);
	});

	it('approveSource(id) posts to sources/{id}/approve with an empty body', () => {
		pluginService.approveSource('src-1');
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/sources/src-1/approve', {});
	});

	it('rejectSource(id) posts to sources/{id}/reject with an empty body', () => {
		pluginService.rejectSource('src-1');
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/sources/src-1/reject', {});
	});

	it('install(input) sends the exact PluginInstallInput body to /install', () => {
		const input = { plugin_id: 'x', source_id: 'y', manifest: '{}' };
		pluginService.install(input);
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/install', input);
	});

	it('install(input) forwards the optional dir when provided', () => {
		const input = { plugin_id: 'x', source_id: 'y', manifest: '{}', dir: '/srv/plugins/x' };
		pluginService.install(input);
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/install', input);
	});

	it('uninstall(id) posts to {id}/uninstall with an empty body', () => {
		pluginService.uninstall('x');
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/x/uninstall', {});
	});

	it('listInstalls() calls post with the installs/list URL and an empty body', () => {
		pluginService.listInstalls();
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/installs/list', {});
	});

	it('update(id, body) includes enabled:true in the body', () => {
		pluginService.update('x', {
			name: 'demo',
			version: '1.0.0',
			author: 'me',
			description: 'desc',
			homepage: 'https://example.com',
			settings: '{}',
			enabled: true,
		});
		expect(postMock).toHaveBeenCalledWith(
			'/admin/plugins/x/update',
			expect.objectContaining({ enabled: true, id: 'x' }),
		);
	});

	it('update(id, body) keeps enabled:false instead of dropping the falsy value', () => {
		pluginService.update('x', { enabled: false });
		expect(postMock).toHaveBeenCalledWith(
			'/admin/plugins/x/update',
			expect.objectContaining({ enabled: false, id: 'x' }),
		);
	});

	it('setEnabled(id, enabled) posts to {id}/status', () => {
		pluginService.setEnabled('x', false);
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/x/status', { enabled: false });
	});

	it('delete(id) posts to {id}/delete with an empty body', () => {
		pluginService.delete('x');
		expect(postMock).toHaveBeenCalledWith('/admin/plugins/x/delete', {});
	});
});

describe('plugin permission codes', () => {
	it('PLUGIN_READ equals plugin:read', () => {
		expect(PermissionCode.PLUGIN_READ).toBe('plugin:read');
	});

	it('PLUGIN_MANAGE equals plugin:manage', () => {
		expect(PermissionCode.PLUGIN_MANAGE).toBe('plugin:manage');
	});
});
