import { afterEach, describe, expect, it, vi } from 'vitest';

import { PollApiError, pollDashboard, pollDogGuard, pollLand } from './pollApi';

afterEach(() => vi.unstubAllGlobals());

describe('poll API', () => {
  it('encodes dashboard and land queries on the embedded origin', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({
        status: {}, guardStatus: {}, bindingStatus: {}, events: [], tsdkEvents: [], automationState: {}, patchStatus: {},
      }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        full: false, revision: 'r 2', lands: [], removedLandIds: [],
      }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const signal = new AbortController().signal;

    await pollDashboard('workspace', signal);
    await pollLand('r 1', signal);

    expect(fetchMock.mock.calls[0][0]).toBe('/farm-api/poll/dashboard?activeTab=workspace');
    expect(fetchMock.mock.calls[1][0]).toBe('/farm-api/poll/land?revision=r+1');
    expect(fetchMock.mock.calls[0][1]).toEqual({ cache: 'no-store', signal });
  });

  it('returns a status-bearing error and rejects malformed responses', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(new Response('{"error":"authorization required"}', { status: 401 }))
      .mockResolvedValueOnce(new Response('{}', { status: 200 })));

    await expect(pollDogGuard(new AbortController().signal)).rejects.toMatchObject<Partial<PollApiError>>({ status: 401 });
    await expect(pollLand('', new AbortController().signal)).rejects.toThrow('Invalid land poll response');
  });

  it('rejects a dashboard response without the dedicated TSDK feed', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      status: {},
      guardStatus: {},
      bindingStatus: {},
      events: [],
      automationState: {},
      patchStatus: {},
    }), { status: 200 })));

    await expect(pollDashboard('workspace', new AbortController().signal))
      .rejects.toThrow('Invalid dashboard poll response');
  });
});
